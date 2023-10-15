// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pebble

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/bloom"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/exp/slices"
)

var (
	_ database.Database = (*Database)(nil)
	_ database.Batch    = (*batch)(nil)
	_ database.Iterator = (*iter)(nil)

	ErrInvalidOperation = errors.New("invalid operation")
)

type Database struct {
	db      *pebble.DB
	metrics *metrics

	// We use an atomic bool for most
	// checks because it is much faster
	// than checking if a channel is closed.
	closing chan struct{}
	closed  utils.Atomic[bool]
}

type Config struct {
	CacheSize                   int // B
	BytesPerSync                int // B
	MemTableStopWritesThreshold int // num tables
	MemTableSize                int // B
	MaxOpenFiles                int
	ConcurrentCompactions       func() int
}

func NewDefaultConfig() Config {
	return Config{
		CacheSize:                   1024 * 1024 * 1024,
		BytesPerSync:                4 * 1024 * 1024, // block size is usually at least 2MB
		MemTableStopWritesThreshold: 8,
		MemTableSize:                16 * 1024 * 1024,
		MaxOpenFiles:                4_096,
		ConcurrentCompactions:       func() int { return 1 },
	}
}

func New(file string, cfg Config) (database.Database, *prometheus.Registry, error) {
	// These default settings are based on https://github.com/ethereum/go-ethereum/blob/master/ethdb/pebble/pebble.go
	d := &Database{closing: make(chan struct{})}
	opts := &pebble.Options{
		Cache:                       pebble.NewCache(int64(cfg.CacheSize)),
		BytesPerSync:                cfg.BytesPerSync,
		MemTableStopWritesThreshold: cfg.MemTableStopWritesThreshold,
		MemTableSize:                cfg.MemTableSize,
		MaxOpenFiles:                cfg.MaxOpenFiles,
		MaxConcurrentCompactions:    cfg.ConcurrentCompactions, // TODO: may want to tweak this?
		Levels:                      make([]pebble.LevelOptions, 7),
		// TODO: add support for adding a custom logger

		EventListener: &pebble.EventListener{
			CompactionBegin: d.onCompactionBegin,
			CompactionEnd:   d.onCompactionEnd,
			WriteStallBegin: d.onWriteStallBegin,
			WriteStallEnd:   d.onWriteStallEnd,
		},
	}
	// Default configuration sourced from:
	// https://github.com/cockroachdb/pebble/blob/master/cmd/pebble/db.go#L76-L86
	for i := 0; i < len(opts.Levels); i++ {
		l := &opts.Levels[i]
		l.BlockSize = 64 * 1024
		l.IndexBlockSize = 256 * 1024
		l.FilterPolicy = bloom.FilterPolicy(10)
		l.FilterType = pebble.TableFilter
		if i > 0 {
			l.TargetFileSize = opts.Levels[i-1].TargetFileSize * 2
		}
	}
	opts.Experimental.ReadSamplingMultiplier = -1 // explicitly disable seek compaction
	registry, metrics, err := newMetrics()
	if err != nil {
		return nil, nil, err
	}
	d.metrics = metrics
	db, err := pebble.Open(file, opts)
	if err != nil {
		return nil, nil, err
	}
	d.db = db
	go d.collectMetrics()
	return d, registry, nil
}

func (db *Database) Close() error {
	defer db.closed.Set(true)
	close(db.closing)
	return updateError(db.db.Close())
}

func (db *Database) HealthCheck(_ context.Context) (interface{}, error) {
	if db.closed.Get() {
		return nil, database.ErrClosed
	}
	return nil, nil
}

// Has returns if the key is set in the database
func (db *Database) Has(key []byte) (bool, error) {
	_, closer, err := db.db.Get(key)
	if err == pebble.ErrNotFound {
		return false, nil
	} else if err != nil {
		return false, updateError(err)
	}
	return true, closer.Close()
}

// Get returns the value the key maps to in the database
func (db *Database) Get(key []byte) ([]byte, error) {
	start := time.Now()
	data, closer, err := db.db.Get(key)
	db.metrics.getLatency.Observe(float64(time.Since(start)))
	if err != nil {
		return nil, updateError(err)
	}
	ret := make([]byte, len(data))
	copy(ret, data)
	return ret, closer.Close()
}

// Put sets the value of the provided key to the provided value
func (db *Database) Put(key []byte, value []byte) error {
	return updateError(db.db.Set(key, value, pebble.Sync))
}

// Delete removes the key from the database
func (db *Database) Delete(key []byte) error {
	return updateError(db.db.Delete(key, pebble.Sync))
}

func (db *Database) Compact(start []byte, limit []byte) error {
	return updateError(db.db.Compact(start, limit, false))
}

// batch is a wrapper around a pebbleDB batch to contain sizes.
type batch struct {
	batch *pebble.Batch
	size  int
}

// NewBatch creates a write/delete-only buffer that is atomically committed to
// the database when write is called
func (db *Database) NewBatch() database.Batch { return &batch{batch: db.db.NewBatch()} }

// Put the value into the batch for later writing
func (b *batch) Put(key, value []byte) error {
	b.size += len(key) + len(value) + 8 // TODO: find byte overhead
	return b.batch.Set(key, value, pebble.Sync)
}

// Delete the key during writing
func (b *batch) Delete(key []byte) error {
	b.size += len(key) + 8 // TODO: find byte overhead
	return b.batch.Delete(key, pebble.Sync)
}

// Size retrieves the amount of data queued up for writing.
func (b *batch) Size() int { return b.size }

// Write flushes any accumulated data to disk.
func (b *batch) Write() error {
	defer b.batch.Close()
	return updateError(b.batch.Commit(pebble.Sync))
}

// Reset resets the batch for reuse.
func (b *batch) Reset() {
	b.batch.Reset()
	b.size = 0
}

// Replay the batch contents.
func (b *batch) Replay(w database.KeyValueWriterDeleter) error {
	reader := b.batch.Reader()
	for {
		kind, k, v, ok := reader.Next()
		if !ok {
			return nil
		}
		switch kind {
		case pebble.InternalKeyKindSet:
			if err := w.Put(k, v); err != nil {
				return err
			}
		case pebble.InternalKeyKindDelete:
			if err := w.Delete(k); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: %v", ErrInvalidOperation, kind)
		}
	}
}

// Inner returns itself
func (b *batch) Inner() database.Batch { return b }

type iter struct {
	db       *Database
	iter     *pebble.Iterator
	setFirst bool

	valid bool
	err   error
}

// NewIterator creates a lexicographically ordered iterator over the database
func (db *Database) NewIterator() database.Iterator {
	return &iter{
		db:   db,
		iter: db.db.NewIter(&pebble.IterOptions{}),
	}
}

// NewIteratorWithStart creates a lexicographically ordered iterator over the
// database starting at the provided key
func (db *Database) NewIteratorWithStart(start []byte) database.Iterator {
	return &iter{
		db:   db,
		iter: db.db.NewIter(&pebble.IterOptions{LowerBound: start}),
	}
}

// bytesPrefix returns key range that satisfy the given prefix.
// This only applicable for the standard 'bytes comparer'.
func bytesPrefix(prefix []byte) *pebble.IterOptions {
	var limit []byte
	for i := len(prefix) - 1; i >= 0; i-- {
		c := prefix[i]
		if c < 0xff {
			limit = make([]byte, i+1)
			copy(limit, prefix)
			limit[i] = c + 1
			break
		}
	}
	return &pebble.IterOptions{LowerBound: prefix, UpperBound: limit}
}

// NewIteratorWithPrefix creates a lexicographically ordered iterator over the
// database ignoring keys that do not start with the provided prefix
func (db *Database) NewIteratorWithPrefix(prefix []byte) database.Iterator {
	return &iter{
		db:   db,
		iter: db.db.NewIter(bytesPrefix(prefix)),
	}
}

// NewIteratorWithStartAndPrefix creates a lexicographically ordered iterator
// over the database starting at start and ignoring keys that do not start with
// the provided prefix.
//
// Prefix should be some key contained within [start] or else the lower bound
// of the iteration will be overwritten with [start].
func (db *Database) NewIteratorWithStartAndPrefix(start, prefix []byte) database.Iterator {
	iterRange := bytesPrefix(prefix)
	if bytes.Compare(start, prefix) == 1 {
		iterRange.LowerBound = start
	}
	return &iter{
		db:   db,
		iter: db.db.NewIter(iterRange),
	}
}

func (it *iter) Next() bool {
	// Short-circuit and set an error if the underlying database has been closed.
	if it.db.closed.Get() {
		it.valid = false
		it.err = database.ErrClosed
		return false
	}

	var hasNext bool
	if !it.setFirst {
		hasNext = it.iter.First()
		it.setFirst = true
	} else {
		hasNext = it.iter.Next()
	}
	it.valid = hasNext
	return hasNext
}

func (it *iter) Error() error {
	if it.err != nil {
		return it.err
	}
	return updateError(it.iter.Error())
}

func (it *iter) Key() []byte {
	if !it.valid {
		return nil
	}
	return slices.Clone(it.iter.Key())
}

func (it *iter) Value() []byte {
	if !it.valid {
		return nil
	}
	return slices.Clone(it.iter.Value())
}

func (it *iter) Release() { it.iter.Close() }

// updateError casts pebble-specific errors to errors that Avalanche VMs expect
// to see (they do not know which type of db may be provided).
func updateError(err error) error {
	switch err {
	case pebble.ErrClosed:
		return database.ErrClosed
	case pebble.ErrNotFound:
		return database.ErrNotFound
	default:
		return err
	}
}

package appenddb

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/linked"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
	"go.uber.org/zap"
	"golang.org/x/exp/mmap"
)

var (
	_ state.Immutable = (*AppendDB)(nil)
	_ state.Mutable   = (*Batch)(nil)
)

const (
	// When mutliple checksums are in a single file (we chose not to rewrite file because
	// many keys unchanged), we opNullify any keys that are no longer active in a batch file (may be
	// active in other batch files still) and compute the next checksum using the base as the previous
	// checksum (rather than re-iterating over the entire file).
	opPut      = uint8(0) // keyLen|key|valueLen|value
	opDelete   = uint8(1) // keyLen|key
	opChecksum = uint8(2) // batch|checksum
	opNullify  = uint8(3) // keyLen|key

	batchBufferSize       = 16 // just need to be big enough for any binary numbers
	minDiskValueSize      = 64
	uselessDividerRecycle = 3
	forceRecycle          = 128 * units.MiB // TODO: make this tuneable
)

type record struct {
	batch uint64

	// Only populated if the value is less than [minDiskValueSize]
	value []byte

	// Location of value in file (does not include
	// operation, key length, key, or value length)
	loc  int64
	size uint32

	// doubly-linked list allows for removals
	prev *record
	next *record
}

func (r *record) Size() int64 {
	if r.loc >= 0 {
		return int64(r.size)
	}
	return int64(len(r.value))
}

type tracker struct {
	db     *AppendDB
	reader *mmap.ReaderAt

	first          *record // first record in list
	last           *record
	pendingNullify []string // never needs to be updated, so we can just keep an array

	checksum     ids.ID
	aliveBytes   int64
	uselessBytes int64 // already includes all nullifies and deletes
}

// we return the record to allow for memory reuse + less map ops
func (t *tracker) Nullify(key string, record *record, addNullify bool) {
	opSize := opPutLenWithValueLen(key, record.Size())
	t.aliveBytes -= opSize
	t.uselessBytes += opSize

	// Remove from linked list
	if record.prev == nil {
		t.first = record.next
	} else {
		record.prev.next = record.next
	}
	if record.next == nil {
		t.last = record.prev
	} else {
		record.next.prev = record.prev
	}

	// We only add to [pendingNullify] if this is called when there is a
	// [Put] or [Delete] op.
	if addNullify {
		t.uselessBytes += opNullifyLen(key)
		t.pendingNullify = append(t.pendingNullify, key)
	}
}

// This sits under the MerkleDB and won't be used
// directly by the VM.
type AppendDB struct {
	logger     logging.Logger
	baseDir    string
	bufferSize int
	historyLen int

	commitLock  sync.RWMutex
	oldestBatch *uint64
	nextBatch   uint64

	keyLock sync.RWMutex
	batches map[uint64]*tracker
	keys    map[string]*record
}

func readOp(reader io.Reader, cursor int64, hasher hash.Hash) (uint8, int64, error) {
	op := make([]byte, consts.Uint8Len)
	if _, err := io.ReadFull(reader, op); err != nil {
		return 0, -1, err
	}
	if _, err := hasher.Write(op); err != nil {
		return 0, -1, err
	}
	cursor++
	return op[0], cursor, nil
}

func readKey(reader io.Reader, cursor int64, hasher hash.Hash) (string, int64, error) {
	op := make([]byte, consts.Uint16Len)
	if _, err := io.ReadFull(reader, op); err != nil {
		return "", -1, err
	}
	if _, err := hasher.Write(op); err != nil {
		return "", -1, err
	}
	cursor += int64(len(op))
	keyLen := binary.BigEndian.Uint16(op)
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(reader, key); err != nil {
		return "", -1, err
	}
	if _, err := hasher.Write(key); err != nil {
		return "", -1, err
	}
	cursor += int64(len(key))
	return string(key), cursor, nil
}

func readPut(reader io.Reader, cursor int64, hasher hash.Hash) (string, []byte, int64, error) {
	key, cursor, err := readKey(reader, cursor, hasher)
	if err != nil {
		return "", nil, -1, err
	}

	// Read value
	op := make([]byte, consts.Uint32Len)
	if _, err := io.ReadFull(reader, op); err != nil {
		return "", nil, -1, err
	}
	if _, err := hasher.Write(op); err != nil {
		return "", nil, -1, err
	}
	cursor += int64(len(op))
	valueLen := binary.BigEndian.Uint32(op)
	value := make([]byte, valueLen)
	if _, err := io.ReadFull(reader, value); err != nil {
		return "", nil, -1, err
	}
	if _, err := hasher.Write(value); err != nil {
		return "", nil, -1, err
	}
	cursor += int64(len(value))
	return key, value, cursor, nil
}

func opPutLen(key string, value []byte) int64 {
	return int64(consts.Uint8Len + consts.Uint16Len + len(key) + consts.Uint32Len + len(value))
}

func opPutLenWithValueLen(key string, valueLen int64) int64 {
	return int64(consts.Uint8Len+consts.Uint16Len+len(key)+consts.Uint32Len) + valueLen
}

func opDeleteLen(key string) int64 {
	return int64(consts.Uint8Len + consts.Uint16Len + len(key))
}

func opNullifyLen(key string) int64 {
	return opDeleteLen(key)
}

func opChecksumLen() int64 {
	return int64(consts.Uint8Len + consts.Uint64Len + ids.IDLen)
}

func readChecksum(reader io.Reader, cursor int64, hasher hash.Hash) (uint64, ids.ID, int64, error) {
	op := make([]byte, consts.Uint64Len)
	if _, err := io.ReadFull(reader, op); err != nil {
		return 0, ids.Empty, -1, err
	}
	if _, err := hasher.Write(op); err != nil {
		return 0, ids.Empty, -1, err
	}
	cursor += int64(len(op))
	batch := binary.BigEndian.Uint64(op)
	op = make([]byte, sha256.Size)
	if _, err := io.ReadFull(reader, op); err != nil {
		return 0, ids.Empty, -1, err
	}
	cursor += int64(len(op))
	return batch, ids.ID(op), cursor, nil
}

func (a *AppendDB) loadBatch(
	path string,
	file uint64,
) error {
	// Load file into buffer
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	reader := bufio.NewReader(f)

	// Read all operations from the file
	var (
		t = &tracker{
			db:             a,
			pendingNullify: []string{},
		}

		fileSize = int64(fi.Size())
		cursor   = int64(0)
		hasher   = sha256.New()
	)
	a.batches[file] = t
	for {
		op, newCursor, err := readOp(reader, cursor, hasher)
		if err != nil {
			return err
		}
		switch op {
		case opPut:
			key, value, newCursor, err := readPut(reader, newCursor, hasher)
			if err != nil {
				return err
			}
			r, ok := a.keys[key]
			if ok {
				a.batches[r.batch].Nullify(key, r, true)
			} else {
				r = &record{}
				a.keys[key] = r
			}
			r.batch = file
			if len(value) >= minDiskValueSize {
				r.value = nil
				r.loc = cursor
				r.size = uint32(len(value))
			} else {
				r.value = value
				r.loc = -1
				r.size = 0
			}
			r.prev = nil
			r.next = nil
			if t.first == nil {
				t.first = r
				t.last = r
			} else {
				r.prev = t.last
				t.last.next = r
				t.last = r
			}
			t.aliveBytes += opPutLen(key, value)
			cursor = newCursor
		case opDelete:
			key, newCursor, err := readKey(reader, newCursor, hasher)
			if err != nil {
				return err
			}
			r, ok := a.keys[key]
			if ok {
				a.batches[r.batch].Nullify(key, r, true)
			} else {
				return errors.New("invalid delete")
			}
			delete(a.keys, key)
			t.uselessBytes += opDeleteLen(key)
			cursor = newCursor
		case opChecksum:
			batch, checksum, newCursor, err := readChecksum(reader, newCursor, hasher)
			if err != nil {
				return err
			}
			if !bytes.Equal(checksum[:], hasher.Sum(nil)) {
				return fmt.Errorf("%w: checksum mismatch", ErrCorrupt)
			}
			t.uselessBytes += opChecksumLen()
			cursor = newCursor

			// Check if we should leave the file
			if cursor == fileSize {
				if batch != file {
					return fmt.Errorf("%w: batch at wrong location %d", ErrCorrupt, batch)
				}
				mf, err := mmap.Open(path)
				if err != nil {
					return err
				}
				t.reader = mf
				t.checksum = checksum
				return nil
			}
			t.pendingNullify = t.pendingNullify[:0]

			// Initialize hasher for next batch (assuming continues)
			hasher = sha256.New()
			if _, err := hasher.Write(checksum[:]); err != nil {
				return err
			}
		case opNullify: // key was updated (either put/delete in another batch file)
			// TODO: ensure happen right after checksum only
			key, newCursor, err := readKey(reader, newCursor, hasher)
			if err != nil {
				return err
			}
			r, ok := a.keys[key]
			if ok {
				a.batches[r.batch].Nullify(key, r, false)
			}
			t.uselessBytes += opNullifyLen(key)
			cursor = newCursor
		default:
			return fmt.Errorf("%w: invalid operation %d", ErrCorrupt, op)
		}
	}
}

// New returns a new AppendDB instance and the ID of the last committed file.
func New(
	log logging.Logger,
	baseDir string,
	initialSize int,
	batchSize int,
	bufferSize int,
	historyLen int, // should not be changed
) (*AppendDB, ids.ID, error) {
	// Iterate over files in directory and put into sorted order
	start := time.Now()
	files := []uint64{}
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if baseDir == path {
			// Don't attempt to load self
			return nil
		}
		if info.IsDir() {
			// Skip anything unexpected
			log.Warn("found unexpected directory", zap.String("path", path))
			return nil
		}
		file, err := strconv.ParseUint(info.Name(), 10, 64)
		if err != nil {
			return err
		}
		files = append(files, file)
		return nil
	})
	if err != nil {
		return nil, ids.Empty, err
	}
	if len(files) > historyLen+1 /* last file could've been in-progress */ {
		log.Warn("found too many files", zap.Int("count", len(files)))
		return nil, ids.Empty, errors.New("too many files")
	}

	// Instantiate DB
	adb := &AppendDB{
		logger:     log,
		baseDir:    baseDir,
		bufferSize: bufferSize,
		historyLen: historyLen,

		keys:    make(map[string]*record, initialSize),
		batches: make(map[uint64]*tracker, historyLen+1),
	}

	// Replay all changes on-disk
	//
	// Note: this will require reading all stored files
	// to reconstruct data. If any files were not fully written or have become corrupt,
	// this will error.
	var (
		lastChecksum ids.ID
		firstBatch   uint64
		lastBatch    uint64
	)
	slices.Sort(files)
	for i, file := range files {
		path := filepath.Join(baseDir, strconv.FormatUint(file, 10))
		if err := adb.loadBatch(path, file); err != nil {
			// We chose not to fix storage if we encounter an error as it
			// could be destructive.
			//
			// TODO: make corruption fix optional/add tool
			log.Warn("could not open batch", zap.String("path", path), zap.Error(err))
			return nil, ids.Empty, err
		}
		if i == 0 {
			firstBatch = file
		}
		lastBatch = file
	}
	log.Info(
		"loaded batches",
		zap.Int("count", len(adb.batches)),
		zap.Int("keys", len(adb.keys)),
		zap.Uint64("first batch", firstBatch),
		zap.Uint64("last batch", lastBatch),
		zap.Stringer("last checksum", lastChecksum),
		zap.Duration("duration", time.Since(start)),
	)
	if len(adb.batches) > 0 {
		adb.oldestBatch = &firstBatch
		adb.nextBatch = lastBatch + 1
	}
	return adb, lastChecksum, nil
}

func (a *AppendDB) get(key string) ([]byte, error) {
	entry, ok := a.keys[key]
	if !ok {
		return nil, database.ErrNotFound
	}
	if entry.loc < 0 {
		return slices.Clone(entry.value), nil
	}
	value := make([]byte, entry.size)
	_, err := a.batches[entry.batch].reader.ReadAt(value, entry.loc)
	return value, err
}

func (a *AppendDB) Get(_ context.Context, key string) ([]byte, error) {
	a.keyLock.RLock()
	defer a.keyLock.RUnlock()

	return a.get(key)
}

func (a *AppendDB) Gets(_ context.Context, keys []string) ([][]byte, []error) {
	a.keyLock.RLock()
	defer a.keyLock.RUnlock()

	values := make([][]byte, len(keys))
	errors := make([]error, len(keys))
	for i, key := range keys {
		values[i], errors[i] = a.get(key)
	}
	return values, errors
}

func (a *AppendDB) Close() error {
	a.commitLock.Lock()
	defer a.commitLock.Unlock()

	for _, file := range a.batches {
		if err := file.reader.Close(); err != nil {
			return err
		}
	}
	a.logger.Info(
		"closing appenddb",
		zap.Int("keys", len(a.keys)),
		zap.Uint64("next batch", a.nextBatch),
		zap.Uint64("batches", uint64(len(a.batches))),
	)
	return nil
}

// Batch is not thread-safe
//
// Batch assumes that keys are inserted in a **deterministic** order
// otherwise the checksum will differ across nodes.
//
// Batch assumes there is only one interaction per key per batch.
type Batch struct {
	a     *AppendDB
	path  string
	batch uint64

	pruneableBatch *uint64
	openWrites     int64 // bytes
	movingPath     string
	startingCursor int64

	hasher hash.Hash
	f      *os.File
	cursor int64

	buf    []byte
	writer *writer

	alive          *linked.Hashmap[string, record]
	pendingNullify *linked.Hashmap[string, any]

	aliveBytes   int64
	uselessBytes int64
}

func (a *AppendDB) NewBatch() (*Batch, error) {
	a.commitLock.Lock()
	batch := a.nextBatch
	path := filepath.Join(a.baseDir, strconv.FormatUint(batch, 10))
	a.nextBatch++
	b := &Batch{
		a:     a,
		path:  path,
		batch: batch,

		hasher: sha256.New(),

		buf: make([]byte, batchBufferSize),
	}
	// If we don't need to recycle, we should create a new hashmap for this
	// batch.
	//
	// TODO: cleanup batch creation errors
	reused, err := b.recycle()
	if err != nil {
		_ = b.f.Close()
		// We don't remove this file because it may be a reuse file that we want to recover
		return nil, err
	}
	if reused {
		return b, nil
	}

	// Setup new batch info
	b.alive, _ = a.preallocAlive.PopLeft()
	b.pendingNullify, _ = a.preallocNullify.PopLeft()
	// Create new file
	f, err := os.Create(b.path)
	if err != nil {
		return nil, err
	}
	b.f = f
	b.writer = newWriter(f, 0, b.a.bufferSize)
	return b, nil
}

func (b *Batch) writeBuffer(value []byte, hash bool) error {
	b.writer.Write(value)
	if hash {
		if _, err := b.hasher.Write(value); err != nil {
			return err
		}
	}
	b.cursor += int64(len(value))
	return nil
}

// recycle reads all keys from the oldest batch
// and puts them into the new file.
//
// A byproduct of this approach is that batches may contain duplicate
// keys (if a key is replaced in this batch), however, we are willing to make
// that tradeoff to begin writing the new batch as soon as possible.
//
// It is safe to call prepare multiple times.
func (b *Batch) recycle() (bool, error) {
	b.a.logger.Debug("recycling previous batch file", zap.Uint64("batch", b.batch))

	// Determine if we should delete the oldest batch
	if b.batch < uint64(b.a.historyLen) {
		return false, nil
	}
	if b.a.oldestBatch == nil {
		return false, nil
	}
	oldestBatch := *b.a.oldestBatch
	if b.batch-uint64(b.a.historyLen) <= oldestBatch {
		// This means that if we are working on batch 2, we will prune batch 0
		// but keep batch 1 around.
		return false, nil
	}

	// Reuse linked hashmaps for both [alive] and [pendingNullify]
	previous := b.a.batches[oldestBatch]
	b.alive = previous.alive
	b.pendingNullify = previous.pendingNullify
	b.pruneableBatch = &oldestBatch

	// Determine if we should continue writing to the file or create a new one
	if previous.aliveBytes < previous.uselessBytes/uselessDividerRecycle || previous.uselessBytes > forceRecycle {
		b.a.logger.Debug("rewriting alive data to a new file", zap.Int64("alive bytes", previous.aliveBytes), zap.Int64("useless bytes", previous.uselessBytes), zap.Uint64("batch", b.batch))

		// Create new file
		f, err := os.Create(b.path)
		if err != nil {
			return false, err
		}
		b.f = f
		b.writer = newWriter(f, 0, b.a.bufferSize)

		// Iterate over alive records and add them to the batch file
		items := b.alive.Len()
		aliveIter := b.alive.NewIterator()
		for i := 0; i < items; i++ {
			// Iterate over all items in [alive] and write them to the batch file
			if !aliveIter.Next() {
				return false, ErrCorrupt
			}

			// Anything left in [alive] should be written
			k := aliveIter.Key()
			value, err := b.a.Get(context.TODO(), k)
			if err != nil {
				return false, err
			}

			// Create a new record for the key in the batch file
			record, err := b.put(k, value)
			if err != nil {
				return false, err
			}
			op := opPutLen(k, value)
			b.openWrites += op
			b.aliveBytes += op
			b.alive.Put(k, record)
		}

		// Nullifications aren't used when rewriting the file but we still need to drop them
		b.pendingNullify.Clear()
		return true, nil
	}

	// Open old batch for writing
	b.a.logger.Debug("appending nullifiers to existing file", zap.Int("count", b.pendingNullify.Len()), zap.Uint64("batch", b.batch))
	b.movingPath = filepath.Join(b.a.baseDir, strconv.FormatUint(oldestBatch, 10))
	f, err := os.OpenFile(b.movingPath, os.O_WRONLY, 0666)
	if err != nil {
		return false, err
	}
	fi, err := f.Stat()
	if err != nil {
		return false, err
	}
	b.f = f
	b.writer = newWriter(f, fi.Size(), b.a.bufferSize)
	b.cursor = int64(previous.reader.Len())
	b.startingCursor = b.cursor
	b.uselessBytes = previous.uselessBytes
	b.aliveBytes = previous.aliveBytes
	if _, err := b.hasher.Write(previous.checksum[:]); err != nil {
		return false, err
	}

	// Add to existing file
	nullifyIter := b.pendingNullify.NewIterator()
	for nullifyIter.Next() {
		key := nullifyIter.Key()
		if err := b.nullify(key); err != nil {
			return false, err
		}
		b.openWrites += opNullifyLen(key)
	}
	b.pendingNullify.Clear()
	return true, nil
}

// TODO: make this a true abort as long as called before Prepare
//
// We must release this lock to shutdown properly
func (b *Batch) Abort() error {
	// Close in-progress file
	if err := b.f.Close(); err != nil {
		return err
	}

	// Cleanup aborted work
	if len(b.movingPath) == 0 {
		if err := os.Remove(b.path); err != nil {
			return err
		}
	} else {
		if err := os.Truncate(b.movingPath, b.startingCursor); err != nil {
			return err
		}
	}

	// Release lock held acquired during [recycle]
	b.a.commitLock.Unlock()
	return nil
}

func (b *Batch) put(key string, value []byte) (record, error) {
	// Check input
	if len(key) > int(consts.MaxUint16) {
		return nil, ErrKeyTooLong
	}

	// Write to disk
	errs := &wrappers.Errs{}
	b.buf = b.buf[:1]
	b.buf[0] = opPut
	errs.Add(b.writeBuffer(b.buf, true))
	b.buf = b.buf[:consts.Uint16Len]
	binary.BigEndian.PutUint16(b.buf, uint16(len(key)))
	errs.Add(b.writeBuffer(b.buf, true))
	errs.Add(b.writeBuffer([]byte(key), true))
	b.buf = b.buf[:consts.Uint32Len]
	binary.BigEndian.PutUint32(b.buf, uint32(len(value)))
	errs.Add(b.writeBuffer(b.buf, true))
	valueStart := b.cursor
	errs.Add(b.writeBuffer(value, true))
	if errs.Err != nil {
		return nil, errs.Err
	}

	// Furnish record for future usage
	//
	// Note: this batch is not mmap'd yet and should not be used until it is.
	var r record
	lv := len(value)
	if lv >= minDiskValueSize {
		r = &diskRecord{
			batch: b.batch,
			loc:   valueStart,
			size:  uint32(lv),
		}
	} else {
		r = &memRecord{
			batch: b.batch,
			value: value,
		}
	}
	return r, nil
}

func (b *Batch) nullify(key string) error {
	// Check input
	if len(key) > int(consts.MaxUint16) {
		return ErrKeyTooLong
	}

	// Write to disk
	errs := &wrappers.Errs{}
	b.buf = b.buf[:1]
	b.buf[0] = opNullify
	errs.Add(b.writeBuffer(b.buf, true))
	b.buf = b.buf[:consts.Uint16Len]
	binary.BigEndian.PutUint16(b.buf, uint16(len(key)))
	errs.Add(b.writeBuffer(b.buf, true))
	errs.Add(b.writeBuffer([]byte(key), true))
	if errs.Err != nil {
		return errs.Err
	}

	// uselessBytes already updated during runtime, don't need
	// to re-calculate here.
	return nil
}

func (b *Batch) checksum() (ids.ID, error) {
	// Write to disk
	errs := &wrappers.Errs{}
	b.buf = b.buf[:1]
	b.buf[0] = opChecksum
	errs.Add(b.writeBuffer(b.buf, true))
	b.buf = b.buf[:consts.Uint64Len]
	binary.BigEndian.PutUint64(b.buf, b.batch)
	errs.Add(b.writeBuffer(b.buf, true))
	checksum := ids.ID(b.hasher.Sum(nil))
	errs.Add(b.writeBuffer(checksum[:], false))
	if errs.Err != nil {
		return ids.Empty, errs.Err
	}

	// We always consider previous checksums useless in the case
	// that we are reusing a file.
	b.uselessBytes += opChecksumLen()
	return checksum, nil
}

// Prepare should be called right before we begin writing to the batch. As soon as
// this function is called, all reads to the database will be blocked.
//
// Prepare returns how many bytes were written to prepare the batch
// and whether we are rewriting an old batch file.
func (b *Batch) Prepare() (int64, bool) {
	b.a.keyLock.Lock()

	if len(b.movingPath) == 0 {
		// Iterate over [alive] and update records for [keys] that were recycled
		aliveIter := b.alive.NewIterator()
		for aliveIter.Next() {
			key, record := aliveIter.Key(), aliveIter.Value()

			// Any past value stored in keys must be the [key] stored in [alive], so
			// we don't need to worry about deleting old values from alive.
			b.a.keys[key] = record
		}
	} else {
		// Migrate all alive keys to the new batch lookup.
		aliveIter := b.alive.NewIterator()
		for aliveIter.Next() {
			record := aliveIter.Value()
			record.SetBatch(b.batch)

			// We don't need to update [keys] because we share a pointer
			// to [record] in [alive].
		}
	}
	return b.openWrites, b.pruneableBatch != nil && len(b.movingPath) == 0
}

func (b *Batch) Put(_ context.Context, key string, value []byte) error {
	record, err := b.put(key, value)
	if err != nil {
		return err
	}
	b.alive.Put(key, record)
	past, ok := b.a.keys[key]
	b.a.keys[key] = record
	b.aliveBytes += opPutLen(key, value)

	// If key did not exist before, just exit
	if !ok {
		return nil
	}

	// If key existed before, update the previous alive record
	//
	// We check to see if the past batch is less
	// than the current batch because we may have just recycled
	// this key and it is already in [alive].
	if pb := past.Batch(); pb < b.batch {
		b.a.batches[pb].Nullify(key)
	} else {
		// In the case that we are putting the same key in the same batch,
		// we will have 2 put records for the same key. We mark this as [uselessBytes]
		// for the purposes of later pruning the value.
		opSize := opPutLenWithValueLen(key, past.Size())
		b.aliveBytes -= opSize
		b.uselessBytes += opSize
	}
	return nil
}

func (b *Batch) Delete(_ context.Context, key string) error {
	// Check input
	if len(key) > int(consts.MaxUint16) {
		return ErrKeyTooLong
	}

	// Check if key even exists
	past, ok := b.a.keys[key]
	if !ok {
		return nil
	}

	// Write to disk
	errs := &wrappers.Errs{}
	b.buf = b.buf[:1]
	b.buf[0] = opDelete
	errs.Add(b.writeBuffer(b.buf, true))
	b.buf = b.buf[:consts.Uint16Len]
	binary.BigEndian.PutUint16(b.buf, uint16(len(key)))
	errs.Add(b.writeBuffer(b.buf, true))
	errs.Add(b.writeBuffer([]byte(key), true))
	if errs.Err != nil {
		return errs.Err
	}

	// Account for useless bytes
	//
	// We check to see if the past batch is less
	// than the current batch because we may have just recycled
	// this key and it is already in [alive].
	if pb := past.Batch(); pb < b.batch {
		b.a.batches[pb].Nullify(key)
	} else {
		b.alive.Delete(key)
		opSize := opPutLenWithValueLen(key, past.Size())
		b.aliveBytes -= opSize
		b.uselessBytes += opSize
	}
	delete(b.a.keys, key)
	b.uselessBytes += opDeleteLen(key)
	return nil
}

func (b *Batch) Get(ctx context.Context, key string) ([]byte, error) {
	return b.a.Get(ctx, key)
}

// Write failure is not expected. If an error is returned,
// it should be treated as fatal.
func (b *Batch) Write() (ids.ID, error) {
	defer func() {
		b.a.keyLock.Unlock()
		b.a.commitLock.Unlock()
	}()

	// Add batch and checksum to file (allows for crash recovery on restart in the case that a file is partially written)
	checksum, err := b.checksum()
	if err != nil {
		return ids.Empty, err
	}

	// Flush all unwritten data to disk
	if err := b.writer.Flush(); err != nil {
		return ids.Empty, fmt.Errorf("%w: could not flush file", err)
	}

	// Close file now that we don't need to write to it anymore
	//
	// Note: we don't require the file to be fsync'd here and assume
	// we can recover the current state on restart.
	if err := b.f.Close(); err != nil {
		return ids.Empty, fmt.Errorf("%w: could not close file", err)
	}

	// Handle old batch cleanup
	if b.pruneableBatch != nil {
		preparedBatch := *b.pruneableBatch
		preparedReader := b.a.batches[preparedBatch]
		if err := preparedReader.reader.Close(); err != nil {
			return ids.Empty, fmt.Errorf("%w: could not close old batch", err)
		}
		if len(b.movingPath) == 0 {
			preparedPath := filepath.Join(b.a.baseDir, strconv.FormatUint(preparedBatch, 10))
			if err := os.Remove(preparedPath); err != nil {
				return ids.Empty, fmt.Errorf("%w: could not remove old batch", err)
			}
		} else {
			// TODO: ensure size is updated correctly
			if err := os.Rename(b.movingPath, b.path); err != nil {
				return ids.Empty, fmt.Errorf("%w: could not rename file", err)
			}
		}
		delete(b.a.batches, preparedBatch)
		oldestBatch := preparedBatch + 1
		b.a.oldestBatch = &oldestBatch
	}

	// Open file for mmap before keys become acessible
	reader, err := mmap.Open(b.path)
	if err != nil {
		// Should never happen
		return ids.Empty, fmt.Errorf("%w: could not mmap new batch", err)
	}

	// Register the batch for reading
	b.a.batches[b.batch] = &tracker{
		reader: reader,

		alive: b.alive,

		pendingNullify: b.pendingNullify,

		checksum:     checksum,
		aliveBytes:   b.aliveBytes,
		uselessBytes: b.uselessBytes,
	}

	// Set oldest batch if haven't done yet
	if b.a.oldestBatch == nil {
		b.a.oldestBatch = &b.batch
	}
	return checksum, nil
}

func (a *AppendDB) Usage() (int, int64 /* alive bytes */, int64 /* useless bytes */) {
	a.commitLock.RLock()
	defer a.commitLock.RUnlock()

	var (
		aliveBytes   int64
		uselessBytes int64
	)
	for _, batch := range a.batches {
		aliveBytes += batch.aliveBytes

		// This includes pending nullifies and may be a little inaccurate as to what
		// is actually on-disk.
		uselessBytes += batch.uselessBytes
	}
	return len(a.keys), aliveBytes, uselessBytes
}

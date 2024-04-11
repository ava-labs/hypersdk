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

type tracker struct {
	db     *AppendDB
	reader *mmap.ReaderAt

	alive *dll

	// Byte adjustments for [pendingNullify] are taken care of
	// prior to use to speed up the rewrite/continue decision in [recycle].
	//
	// pendingNullify is just used to track pending
	// changes to keys for the next batch.
	//
	// Items never need to be deleted from [pendingNullify], so we can just keep
	// an array
	pendingNullify []string

	checksum     ids.ID
	aliveBytes   int64
	uselessBytes int64 // already includes all nullifies and deletes
}

// we return the record to allow for memory reuse + less map ops
func (t *tracker) Remove(record *record, addNullify bool) {
	opSize := opPutLenWithValueLen(record.key, record.Size())
	t.aliveBytes -= opSize
	t.uselessBytes += opSize

	// Remove from linked list
	t.alive.Remove(record)

	// We only add to [pendingNullify] if this is called when there is a
	// [Put] or [Delete] op.
	if addNullify {
		t.uselessBytes += opNullifyLen(record.key)
		t.pendingNullify = append(t.pendingNullify, record.key)
	}
}

// This sits under the MerkleDB and won't be used
// directly by the VM.
type AppendDB struct {
	logger      logging.Logger
	baseDir     string
	nullifySize int
	bufferSize  int
	historyLen  int

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
			alive:          &dll{},
			pendingNullify: make([]string, 0, a.nullifySize),
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
				a.batches[r.batch].Remove(r, r.batch < file)
				r.batch = file
			} else {
				r = &record{batch: file, key: key}
				a.keys[key] = r
			}
			if len(value) >= minDiskValueSize {
				r.value = nil
				r.loc = cursor
				r.size = uint32(len(value))
			} else {
				r.value = value
				r.loc = -1
				r.size = 0
			}
			t.alive.Add(r)
			t.aliveBytes += opPutLen(key, value)
			cursor = newCursor
		case opDelete:
			key, newCursor, err := readKey(reader, newCursor, hasher)
			if err != nil {
				return err
			}
			r, ok := a.keys[key]
			if ok {
				// Drop this key from our tracking and mark the operation as useless
				a.batches[r.batch].Remove(r, r.batch < file)
				delete(a.keys, key)
			}
			// It is ok if the key doesn't exist when processing a delete op as we may be processing
			// an old batch that has been reused.

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

			// Refresh [pendingNullify] to avoid hanging on to references to old
			// keys.
			if c := cap(t.pendingNullify); c > a.nullifySize {
				a.nullifySize = c
			}
			t.pendingNullify = make([]string, 0, a.nullifySize)

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
			if !ok {
				return errors.New("nullifying unset key")
			}

			// opNullify means this key will be set later, so let's delete our reference for now
			a.batches[r.batch].Remove(r, false)
			delete(a.keys, key)
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
		logger:      log,
		baseDir:     baseDir,
		nullifySize: batchSize,
		bufferSize:  bufferSize,
		historyLen:  historyLen,

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
		lastChecksum = adb.batches[file].checksum
	}
	if len(adb.batches) > 0 {
		adb.oldestBatch = &firstBatch
		adb.nextBatch = lastBatch + 1
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

	t *tracker
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

		t: &tracker{db: a},
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

	// Create new file
	f, err := os.Create(b.path)
	if err != nil {
		return nil, err
	}
	b.f = f
	b.writer = newWriter(f, 0, b.a.bufferSize)
	b.t.alive = &dll{}
	b.t.pendingNullify = make([]string, 0, b.a.nullifySize)
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
	b.pruneableBatch = &oldestBatch
	b.t.pendingNullify = previous.pendingNullify

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
		b.t.alive = &dll{}
		iter := previous.alive.Iterator()
		for next := iter.Next(); next != nil; next = iter.Next() {
			value, err := b.a.Get(context.TODO(), next.key)
			if err != nil {
				return false, err
			}

			// Create a new record for the key in the batch file
			//
			// This is the only time we can't reuse a record and need
			// to re-add to the map (this is to keep a set of pending records
			// prior to grabbing the key lock).
			r := &record{batch: b.batch, key: next.key}
			start, err := b.writePut(next.key, value)
			if err != nil {
				return false, err
			}
			if len(value) >= minDiskValueSize {
				r.loc = start
				r.size = uint32(len(value))
			} else {
				r.value = value
				r.loc = -1
			}
			op := opPutLen(next.key, value)
			b.openWrites += op
			b.t.aliveBytes += op
			b.t.alive.Add(r)
		}

		// Nullifications aren't used when rewriting the file but we still need to drop them
		b.t.pendingNullify = make([]string, 0, b.a.nullifySize)
		return true, nil
	}

	// Open old batch for writing
	b.a.logger.Debug("appending nullifiers to existing file", zap.Int("count", len(b.t.pendingNullify)), zap.Uint64("batch", b.batch))
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
	b.t.uselessBytes = previous.uselessBytes
	b.t.aliveBytes = previous.aliveBytes
	if _, err := b.hasher.Write(previous.checksum[:]); err != nil {
		return false, err
	}

	// Add to existing file
	b.t.alive = previous.alive
	for _, key := range b.t.pendingNullify {
		if err := b.writeNullify(key); err != nil {
			return false, err
		}
		// We already incremented [uselessBytes] when calling [tracker.Remove], so we don't
		// call it again here.
		b.openWrites += opNullifyLen(key)
	}

	// If we are nullifying more data than expected, begin allocating larger slices
	// to prevent growing during batch.
	if c := cap(b.t.pendingNullify); c > b.a.nullifySize {
		b.a.nullifySize = c
	}
	b.t.pendingNullify = make([]string, 0, b.a.nullifySize)
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
	b.a.nextBatch--

	// Release lock held acquired during [recycle]
	b.a.commitLock.Unlock()
	b.a.logger.Debug("aborted batch", zap.Uint64("batch", b.batch))
	return nil
}

func (b *Batch) writePut(key string, value []byte) (int64, error) {
	// Check input
	if len(key) > int(consts.MaxUint16) {
		return -1, ErrKeyTooLong
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
	return valueStart, errs.Err
}

func (b *Batch) writeNullify(key string) error {
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
	return errs.Err
}

func (b *Batch) writeChecksum() (ids.ID, error) {
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
	return checksum, errs.Err
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
		iter := b.t.alive.Iterator()
		for next := iter.Next(); next != nil; next = iter.Next() {
			b.a.keys[next.key] = next
			b.a.logger.Debug("updating key to new record", zap.String("key", next.key), zap.Uint64("batch", next.batch))
		}
	} else {
		// Migrate all alive keys to the new batch lookup.
		iter := b.t.alive.Iterator()
		for next := iter.Next(); next != nil; next = iter.Next() {
			next.batch = b.batch
			b.a.logger.Debug("migrating key to new batch", zap.String("key", next.key), zap.Uint64("prev", next.batch), zap.Uint64("new", b.batch))
		}
	}

	// Setup tracker for reference by this batch
	//
	// We wait to do this until here because we need to hold
	// [keyLock] to ensure that [b.a.batches] is not referenced
	// at the same time.
	b.a.batches[b.batch] = b.t
	return b.openWrites, b.pruneableBatch != nil && len(b.movingPath) == 0
}

func (b *Batch) Put(_ context.Context, key string, value []byte) error {
	start, err := b.writePut(key, value)
	if err != nil {
		return err
	}
	past, ok := b.a.keys[key]
	if ok {
		b.a.batches[past.batch].Remove(past, past.batch < b.batch)
		past.batch = b.batch

		// We avoid updating [keys] when just updating the value of a [record]
	} else {
		past = &record{batch: b.batch, key: key}
		b.a.keys[key] = past
	}
	if len(value) >= minDiskValueSize {
		past.value = nil
		past.loc = start
		past.size = uint32(len(value))
	} else {
		past.value = value
		past.loc = -1
		past.size = 0
	}
	b.t.alive.Add(past)
	b.t.aliveBytes += opPutLen(key, value)
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
		b.a.logger.Debug("attempted to delete non-existent key", zap.String("key", key))
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
	b.a.batches[past.batch].Remove(past, past.batch < b.batch)
	delete(b.a.keys, key)
	b.t.uselessBytes += opDeleteLen(key)
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
	checksum, err := b.writeChecksum()
	if err != nil {
		return ids.Empty, err
	}
	// We always consider previous checksums useless in the case
	// that we are reusing a file.
	b.t.uselessBytes += opChecksumLen()

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
	b.t.reader = reader
	b.t.checksum = checksum

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
	for n, batch := range a.batches {
		aliveBytes += batch.aliveBytes

		// This includes pending nullifies and may be a little inaccurate as to what
		// is actually on-disk.
		uselessBytes += batch.uselessBytes
		a.logger.Debug(
			"batch usage",
			zap.Uint64("batch", n),
			zap.Int64("alive", batch.aliveBytes),
			zap.Int64("useless", batch.uselessBytes),
			zap.Int64("total", batch.aliveBytes+batch.uselessBytes),
		)
	}
	return len(a.keys), aliveBytes, uselessBytes
}

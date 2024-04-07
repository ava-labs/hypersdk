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
	"github.com/ava-labs/avalanchego/utils/buffer"
	"github.com/ava-labs/avalanchego/utils/linked"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
	"github.com/dustin/go-humanize"
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

	batchBufferSize  = 16 * units.KiB
	minDiskValueSize = 512
	forceRecycle     = 128 * units.MiB // TODO: make this tuneable
)

type record struct {
	batch uint64

	value []byte

	// Location of value in file (does not include
	// operation, key length, key, or value length)
	loc  int64
	size uint32
}

func (r *record) valueSize() uint32 {
	if r.loc >= 0 {
		return r.size
	}
	return uint32(len(r.value))
}

type tracker struct {
	reader *mmap.ReaderAt

	alive *linked.Hashmap[string, *record]

	// Byte adjustments for [pendingNullify] are taken care of
	// prior to use to speed up the rewrite/continue decision in [recycle].
	//
	// pendingNullify is just used to track pending
	// changes to keys for the next batch.
	pendingNullify *linked.Hashmap[string, uint64]

	checksum     ids.ID
	aliveBytes   uint64
	uselessBytes uint64 // already includes all nullifies and deletes
}

func (t *tracker) Nullify(key string) {
	// Get bytes of nullified object
	record, ok := t.alive.Get(key)
	if !ok {
		// Exit loudly as this could be a sign of a bug or corruption
		panic("nullifying key that is not in batch")
	}
	opSize := opPutLenWithValueLen(key, record.valueSize())
	t.alive.Delete(key)
	t.aliveBytes -= opSize
	t.uselessBytes += opSize

	// Mark key for nullification
	t.pendingNullify.Put(key, opSize)
	t.uselessBytes += opNullifyLen(key)
}

// This sits under the MerkleDB and won't be used
// directly by the VM.
type AppendDB struct {
	logger     logging.Logger
	baseDir    string
	bufferSize int
	historyLen int

	preallocAlive   buffer.Deque[*linked.Hashmap[string, *record]]
	preallocNullify buffer.Deque[*linked.Hashmap[string, uint64]]

	commitLock  sync.RWMutex
	oldestBatch *uint64
	nextBatch   uint64
	size        uint64

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

func opPutLen(key string, value []byte) uint64 {
	return uint64(consts.Uint8Len + consts.Uint16Len + len(key) + consts.Uint32Len + len(value))
}

func opPutLenWithValueLen(key string, valueLen uint32) uint64 {
	return uint64(consts.Uint8Len+consts.Uint16Len+len(key)+consts.Uint32Len) + uint64(valueLen)
}

func opDeleteLen(key string) uint64 {
	return uint64(consts.Uint8Len + consts.Uint16Len + len(key))
}

func opNullifyLen(key string) uint64 {
	return opDeleteLen(key)
}

func opChecksumLen() uint64 {
	return uint64(consts.Uint8Len + consts.Uint64Len + ids.IDLen)
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

func loadBatch(
	path string,
	alive *linked.Hashmap[string, *record],
	file uint64,
) (
	*tracker,
	*linked.Hashmap[string, *record], // change order is important
	error,
) {
	// Load file into buffer
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	reader := bufio.NewReader(f)

	// Read all operations from the file
	var (
		fileSize = int64(fi.Size())
		cursor   = int64(0)

		hasher = sha256.New()

		changes      = linked.NewHashmap[string, *record]()
		aliveBytes   uint64
		uselessBytes uint64
	)
	for {
		op, newCursor, err := readOp(reader, cursor, hasher)
		if err != nil {
			return nil, nil, err
		}
		switch op {
		case opPut:
			key, value, newCursor, err := readPut(reader, newCursor, hasher)
			if err != nil {
				return nil, nil, err
			}
			record := &record{batch: file}
			if len(value) >= minDiskValueSize {
				record.loc = cursor
				record.size = uint32(len(value))
			} else {
				record.value = value
				record.loc = -1
			}
			previousRecord, ok := alive.Get(key)
			if ok {
				// This could happen if we put a key in the same batch twice (when reading
				// keys during prepare).
				opSize := opPutLenWithValueLen(key, previousRecord.valueSize())
				aliveBytes -= opSize
				uselessBytes += opSize
			}
			alive.Put(key, record)
			changes.Put(key, record)
			aliveBytes += opPutLen(key, value)
			cursor = newCursor
		case opDelete:
			key, newCursor, err := readKey(reader, newCursor, hasher)
			if err != nil {
				return nil, nil, err
			}
			if past, ok := alive.Get(key); ok {
				opSize := opPutLenWithValueLen(key, past.valueSize())
				aliveBytes -= opSize
				uselessBytes += opSize
				alive.Delete(key)
			}
			changes.Put(key, nil)
			uselessBytes += opDeleteLen(key)
			cursor = newCursor
		case opChecksum:
			batch, checksum, newCursor, err := readChecksum(reader, newCursor, hasher)
			if err != nil {
				return nil, nil, err
			}
			if !bytes.Equal(checksum[:], hasher.Sum(nil)) {
				return nil, nil, fmt.Errorf("%w: checksum mismatch", ErrCorrupt)
			}
			uselessBytes += opChecksumLen()
			cursor = newCursor

			// Check if we should leave the file
			if cursor == fileSize {
				if batch != file {
					return nil, nil, fmt.Errorf("%w: batch at wrong location %d", ErrCorrupt, batch)
				}
				mf, err := mmap.Open(path)
				if err != nil {
					return nil, nil, err
				}
				return &tracker{
					reader: mf,

					alive: alive,

					checksum: checksum,
					// pendingNullify is populated by caller

					aliveBytes:   aliveBytes,
					uselessBytes: uselessBytes,
				}, changes, nil
			}

			// Initialize hasher for next batch (assuming continues)
			hasher = sha256.New()
			if _, err := hasher.Write(checksum[:]); err != nil {
				return nil, nil, err
			}
		case opNullify: // key was updated (either put/delete in another batch file)
			key, newCursor, err := readKey(reader, newCursor, hasher)
			if err != nil {
				return nil, nil, err
			}
			if past, ok := alive.Get(key); ok {
				opSize := opPutLenWithValueLen(key, past.valueSize())
				aliveBytes -= opSize
				uselessBytes += opSize
				alive.Delete(key)
			}
			changes.Delete(key) // if we were going to do something with key, don't
			uselessBytes += opNullifyLen(key)
			cursor = newCursor
		default:
			return nil, nil, fmt.Errorf("%w: invalid operation %d", ErrCorrupt, op)
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

	// Provision all batch trackers ahead of time
	preallocs := (historyLen + 1) // keep history + current
	preallocAlive := buffer.NewUnboundedDeque[*linked.Hashmap[string, *record]](preallocs)
	preallocNullify := buffer.NewUnboundedDeque[*linked.Hashmap[string, uint64]](preallocs)
	for i := 0; i < preallocs; i++ {
		preallocAlive.PushRight(linked.NewHashmapWithSize[string, *record](batchSize))
		preallocNullify.PushRight(linked.NewHashmapWithSize[string, uint64](batchSize))
	}

	// Replay all changes on-disk
	//
	// Note: this will require reading all stored files
	// to reconstruct data. If any files were not fully written or have become corrupt,
	// this will error.
	var (
		keys    = make(map[string]*record, initialSize)
		batches = make(map[uint64]*tracker, historyLen+1)
		size    uint64

		lastChecksum ids.ID
		firstBatch   uint64
		lastBatch    uint64
	)
	slices.Sort(files)
	for i, file := range files {
		path := filepath.Join(baseDir, strconv.FormatUint(file, 10))
		alive, ok := preallocAlive.PopLeft()
		if !ok {
			log.Warn("exceeded prealloc alive allocation")
			alive = linked.NewHashmapWithSize[string, *record](batchSize)
		}
		track, changes, err := loadBatch(path, alive, file)
		if err != nil {
			// We chose not to fix storage if we encounter an error as it
			// could be destructive.
			//
			// TODO: make corruption fix optional/add tool
			log.Warn("could not open batch", zap.String("path", path), zap.Error(err))
			return nil, ids.Empty, err
		}
		nullify, ok := preallocNullify.PopLeft()
		if !ok {
			log.Warn("exceeded prealloc nullify allocation")
			nullify = linked.NewHashmapWithSize[string, uint64](batchSize)
		}
		track.pendingNullify = nullify
		changeIter := changes.NewIterator()
		for changeIter.Next() {
			key, entry := changeIter.Key(), changeIter.Value()
			past, ok := keys[key]
			if ok {
				batches[past.batch].Nullify(key)
			}

			// Update entry
			if entry == nil {
				delete(keys, key)
			} else {
				keys[key] = entry
			}
		}
		if i == 0 {
			firstBatch = file
		}
		batches[file] = track
		lastChecksum = track.checksum
		lastBatch = file
		size += uint64(track.reader.Len())
	}
	log.Info(
		"loaded batches",
		zap.Int("count", len(batches)),
		zap.Int("keys", len(keys)),
		zap.Uint64("first batch", firstBatch),
		zap.Uint64("last batch", lastBatch),
		zap.Stringer("last checksum", lastChecksum),
		zap.String("size", humanize.Bytes(size)),
		zap.Duration("duration", time.Since(start)),
	)
	adb := &AppendDB{
		logger:     log,
		baseDir:    baseDir,
		bufferSize: bufferSize,
		historyLen: historyLen,

		preallocAlive:   preallocAlive,
		preallocNullify: preallocNullify,

		batches: batches,
		keys:    keys,
	}
	if len(batches) > 0 {
		adb.oldestBatch = &firstBatch
		adb.nextBatch = lastBatch + 1
		adb.size = size
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
		zap.String("size", humanize.Bytes(a.size)),
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
	openWrites     uint64 // bytes
	movingPath     string
	startingCursor int64

	hasher hash.Hash
	f      *os.File
	cursor int64

	buf    []byte
	writer *bufio.Writer

	alive          *linked.Hashmap[string, *record]
	pendingNullify *linked.Hashmap[string, uint64]

	aliveBytes   uint64
	uselessBytes uint64
}

// TODO: cleanup batch creation errors
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
	newBatch, err := b.recycle()
	if err != nil {
		_ = b.f.Close()
		// We don't remove this file because it may be a reuse file that we want to recover
		return nil, err
	}
	if newBatch {
		b.alive, _ = a.preallocAlive.PopLeft()
		b.pendingNullify, _ = a.preallocNullify.PopLeft()
	}
	return b, nil
}

func (b *Batch) growBuffer(size int) {
	if cap(b.buf) < size {
		b.buf = make([]byte, size, size*2)
	} else {
		b.buf = b.buf[:size]
	}
}

// recycle reads all keys from the oldest batch
// and puts them into the new file.
//
// A byproduct of this approach is that batches may contain duplicate
// keys (if a key is replaced in this batch), however, we are willing to make
// that tradeoff to begin writing the new batch as soon as possible.
//
// It is safe to call prepare multiple times.
func (b *Batch) recycle() (bool /* new batch */, error) {
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

	// Reuse alive tracker we have been keeping up-to-date
	previous := b.a.batches[oldestBatch]
	b.alive = previous.alive
	b.pendingNullify = previous.pendingNullify

	// Determine if we should continue writing to the file or create a new one
	b.pruneableBatch = &oldestBatch
	if previous.aliveBytes < previous.uselessBytes || previous.uselessBytes > forceRecycle {
		// Create new file
		f, err := os.Create(b.path)
		if err != nil {
			return false, err
		}
		b.f = f
		b.writer = bufio.NewWriterSize(f, b.a.bufferSize)

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
			b.openWrites += opPutLen(k, value)
			b.alive.Put(k, record)
		}
		return true, nil
	}

	// Open old batch file
	b.movingPath = filepath.Join(b.a.baseDir, strconv.FormatUint(oldestBatch, 10))
	f, err := os.Open(b.movingPath)
	if err != nil {
		return false, err
	}
	b.f = f
	b.writer = bufio.NewWriterSize(f, b.a.bufferSize)
	b.cursor = int64(previous.reader.Len())
	b.startingCursor = b.cursor
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

func (b *Batch) put(key string, value []byte) (*record, error) {
	// Check input
	if len(key) > int(consts.MaxUint16) {
		return nil, ErrKeyTooLong
	}

	// Create operation to write
	valueStart := b.cursor + int64(consts.Uint8Len+consts.Uint16Len+len(key)+consts.Uint32Len)
	l := consts.Uint8Len + consts.Uint16Len + len(key) + consts.Uint32Len + len(value)
	b.growBuffer(l)
	b.buf[0] = opPut
	binary.BigEndian.PutUint16(b.buf[1:], uint16(len(key)))
	copy(b.buf[1+consts.Uint16Len:], key)
	binary.BigEndian.PutUint32(b.buf[1+consts.Uint16Len+len(key):], uint32(len(value)))
	copy(b.buf[1+consts.Uint16Len+len(key)+consts.Uint32Len:], value)

	// Write to disk
	if _, err := b.writer.Write(b.buf); err != nil {
		return nil, err
	}
	if _, err := b.hasher.Write(b.buf); err != nil {
		return nil, err
	}
	b.cursor += int64(l)

	// Furnish record for future usage
	//
	// Note: this batch is not mmap'd yet and should not be used until it is.
	r := &record{batch: b.batch}
	lv := len(value)
	if lv >= minDiskValueSize {
		r.loc = valueStart
		r.size = uint32(lv)
	} else {
		r.value = value
		r.loc = -1
	}
	return r, nil
}

func (b *Batch) nullify(key string) error {
	// Check input
	if len(key) > int(consts.MaxUint16) {
		return ErrKeyTooLong
	}

	// Create operation to write
	l := consts.Uint8Len + consts.Uint16Len + len(key)
	b.growBuffer(l)
	b.buf[0] = opNullify
	binary.BigEndian.PutUint16(b.buf[1:], uint16(len(key)))
	copy(b.buf[1+consts.Uint16Len:], key)

	// Write to disk
	if _, err := b.writer.Write(b.buf); err != nil {
		return err
	}
	if _, err := b.hasher.Write(b.buf); err != nil {
		return err
	}
	b.cursor += int64(l)
	return nil
}

// Prepare should be called right before we begin writing to the batch. As soon as
// this function is called, all reads to the database will be blocked.
//
// Prepare returns how many bytes were written to prepare the batch
// and whether or not we are reusing an old batch file.
func (b *Batch) Prepare() (uint64, bool) {
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
			record.batch = b.batch

			// We don't need to update [keys] because we share a pointer
			// to [record] in [alive].
		}
	}
	return b.openWrites, len(b.movingPath) > 0
}

func (b *Batch) Put(_ context.Context, key string, value []byte) error {
	record, err := b.put(key, value)
	if err != nil {
		return err
	}
	b.alive.Put(key, record)

	// If key never existed before, just exit
	past, ok := b.a.keys[key]
	if !ok {
		return nil
	}

	// If key existed before, update the previous alive record
	//
	// We check to see if the past batch is less
	// than the current batch because we may have just recycled
	// this key and it is already in [alive].
	if past.batch < b.batch {
		b.a.batches[past.batch].Nullify(key)
	} else {
		// In the case that we are putting the same key in the same batch,
		// we will have 2 put records for the same key. We mark this as [uselessBytes]
		// for the purposes of later pruning the value.
		opSize := opPutLenWithValueLen(key, past.valueSize())
		b.aliveBytes -= opSize
		b.uselessBytes += opSize
	}
	b.a.keys[key] = record
	b.aliveBytes += opPutLen(key, value)
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

	// Create operation to write
	l := consts.Uint8Len + consts.Uint16Len + len(key)
	b.growBuffer(l)
	b.buf[0] = opDelete
	binary.BigEndian.PutUint16(b.buf[1:], uint16(len(key)))
	copy(b.buf[1+consts.Uint16Len:], key)

	// Write to disk
	if _, err := b.writer.Write(b.buf); err != nil {
		return err
	}
	if _, err := b.hasher.Write(b.buf); err != nil {
		return err
	}
	b.cursor += int64(l)

	// Account for useless bytes
	//
	// We check to see if the past batch is less
	// than the current batch because we may have just recycled
	// this key and it is already in [alive].
	if past.batch < b.batch {
		b.a.batches[past.batch].Nullify(key)
	} else {
		b.alive.Delete(key)
		opSize := opPutLenWithValueLen(key, past.valueSize())
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

	// If we already encountered an error, return that immediately
	if b.err != nil {
		return ids.Empty, b.err
	}

	// Add checksum to file (allows for crash recovery on restart in the case that a file is partially written)
	checksum := b.hasher.Sum(nil)
	if _, err := b.writer.Write(checksum); err != nil {
		return ids.Empty, err
	}

	// Flush all unwritten data to disk
	if err := b.writer.Flush(); err != nil {
		return ids.Empty, err
	}

	// Close file now that we don't need to write to it anymore
	//
	// Note: we don't require the file to be fsync'd here and assume
	// we can recover the current state on restart.
	if err := b.f.Close(); err != nil {
		return ids.Empty, err
	}

	// Open file for mmap before keys become acessible
	reader, err := mmap.Open(b.path)
	if err != nil {
		// Should never happen
		return ids.Empty, err
	}
	b.a.size += uint64(reader.Len())

	// Register the batch for reading
	b.a.batches[b.batch] = &tracker{reader: reader, alive: b.alive}

	// Delete old batch once keys are updated to new batch
	if b.pruneableBatch != nil {
		preparedBatch := *b.pruneableBatch
		preparedReader := b.a.batches[preparedBatch]
		preparedSize := uint64(preparedReader.reader.Len())
		if err := preparedReader.reader.Close(); err != nil {
			b.a.logger.Error("could not close old batch", zap.Uint64("batch", preparedBatch), zap.Error(err))
		}
		preparedPath := filepath.Join(b.a.baseDir, strconv.FormatUint(preparedBatch, 10))
		if err := os.Remove(preparedPath); err != nil {
			b.a.logger.Error("could not remove old batch", zap.Uint64("batch", preparedBatch), zap.Error(err))
		}
		delete(b.a.batches, preparedBatch)
		b.a.size -= preparedSize
		oldestBatch := preparedBatch + 1
		b.a.oldestBatch = &oldestBatch
	}
	if b.a.oldestBatch == nil {
		b.a.oldestBatch = &b.batch
	}
	return ids.ID(checksum), nil
}

func (a *AppendDB) Usage() (int, uint64) {
	a.commitLock.RLock()
	defer a.commitLock.RUnlock()

	return len(a.keys), a.size
}

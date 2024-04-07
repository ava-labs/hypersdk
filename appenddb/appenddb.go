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
	"github.com/ava-labs/avalanchego/utils/set"
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

	lastChecksum  ids.ID
	pendingNulify set.Set[string]

	aliveBytes   uint64
	uselessBytes uint64 // already includes all nulifies and deletes
}

func (t *tracker) Nulify(key string) {
	// Get bytes of nulified object
	record, _ := t.alive.Get(key)
	size := uint32(consts.Uint8Len+consts.Uint16Len+len(key)+consts.Uint32Len) + record.valueSize()
	t.alive.Delete(key)
	t.aliveBytes -= uint64(size)

	// Mark key for nulification
	t.pendingNulify.Add(key)
	t.uselessBytes += uint64(size) + uint64(consts.Uint8Len+consts.Uint16Len+len(key))
}

// This sits under the MerkleDB and won't be used
// directly by the VM.
type AppendDB struct {
	logger     logging.Logger
	baseDir    string
	bufferSize int
	historyLen int

	preallocAlive buffer.Deque[*linked.Hashmap[string, *record]]

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
	set.Set[string],
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
		hasher   = sha256.New()

		deletes      = set.NewSet[string](1_024)
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
			alive.Put(key, record)
			aliveBytes += uint64(consts.Uint8Len + consts.Uint16Len + len(key) + consts.Uint32Len + len(value))
			cursor = newCursor
		case opDelete:
			key, newCursor, err := readKey(reader, newCursor, hasher)
			if err != nil {
				return nil, nil, err
			}
			if past, ok := alive.Get(key); ok {
				opSize := uint64(uint32(consts.Uint8Len+consts.Uint16Len+len(key)+consts.Uint32Len) + past.valueSize())
				aliveBytes -= opSize
				uselessBytes += opSize
				alive.Delete(key)
			}
			deletes.Add(key)
			uselessBytes += uint64(consts.Uint8Len + consts.Uint16Len + len(key))
			cursor = newCursor
		case opChecksum:
			batch, checksum, newCursor, err := readChecksum(reader, newCursor, hasher)
			if err != nil {
				return nil, nil, err
			}
			if !bytes.Equal(checksum[:], hasher.Sum(nil)) {
				return nil, nil, fmt.Errorf("%w: checksum mismatch", ErrCorrupt)
			}
			uselessBytes += uint64(consts.Uint8Len + consts.Uint64Len + sha256.Size)
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

					lastChecksum:  checksum,
					pendingNulify: set.NewSet[string](1_024),

					aliveBytes:   aliveBytes,
					uselessBytes: uselessBytes,
				}, deletes, nil
			}

			// Initialize hasher for next batch (assuming continues)
			hasher = sha256.New()
			if _, err := hasher.Write(checksum[:]); err != nil {
				return nil, nil, err
			}
		case opNullify:
			key, newCursor, err := readKey(reader, newCursor, hasher)
			if err != nil {
				return nil, nil, err
			}
			if past, ok := alive.Get(key); ok {
				opSize := uint64(uint32(consts.Uint8Len+consts.Uint16Len+len(key)+consts.Uint32Len) + past.valueSize())
				aliveBytes -= opSize
				uselessBytes += opSize
				alive.Delete(key)
			}
			deletes.Remove(key)
			uselessBytes += uint64(consts.Uint8Len + consts.Uint16Len + len(key))
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
	preallocs := historyLen + 1 // keep history + current
	preallocAlive := buffer.NewUnboundedDeque[*linked.Hashmap[string, *record]](preallocs)
	for i := 0; i < preallocs; i++ {
		preallocAlive.PushRight(linked.NewHashmapWithSize[string, *record](batchSize))
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
			log.Warn("exceeded prealloc allocation") // can happen if we don't delete files we expected
			alive = linked.NewHashmapWithSize[string, *record](batchSize)
		}
		checksum, deletes, err := loadBatch(path, alive, file)
		if err != nil {
			// We chose not to fix storage if we encounter an error
			//
			// TODO: make corruption fix optional/add tool
			log.Warn("could not open batch", zap.String("path", path), zap.Error(err))
			return nil, ids.Empty, err
		}
		aliveIter := alive.NewIterator()
		for aliveIter.Next() {
			key, entry := aliveIter.Key(), aliveIter.Value()
			past, ok := keys[key]

			// Delete old value from previous batch before reuse
			if ok {
				batches[past.batch].alive.Delete(key)
			}

			// Update entry
			if entry == nil {
				delete(keys, key)
				alive.Delete(key) // only track keys that are alive
			} else {
				if ok {
					// Reuse record if already exists
					past.batch = entry.batch
					past.value = entry.value
					past.loc = entry.loc
					past.size = entry.size
				} else {
					keys[key] = entry
				}
			}
		}
		if i == 0 {
			firstBatch = file
		}
		reader, err := mmap.Open(path)
		if err != nil {
			log.Warn("could not mmap batch", zap.String("path", path), zap.Error(err))
			return nil, ids.Empty, err
		}
		batches[file] = &tracker{reader: reader, alive: alive}
		lastChecksum = checksum
		lastBatch = file
		size += uint64(reader.Len())

		// TODO: delete files that exceed our history on restart (would be part of crash recovery)
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

		preallocAlive: preallocAlive,

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
	recycled       int

	hasher hash.Hash
	f      *os.File
	cursor int64

	buf    []byte
	writer *bufio.Writer
	alive  *linked.Hashmap[string, *record]

	err error
}

func (a *AppendDB) NewBatch() (*Batch, error) {
	a.commitLock.Lock()
	batch := a.nextBatch
	path := filepath.Join(a.baseDir, strconv.FormatUint(batch, 10))
	f, err := os.Create(path)
	if err != nil {
		a.commitLock.Unlock()
		return nil, err
	}
	a.nextBatch++
	b := &Batch{
		a:     a,
		path:  path,
		batch: batch,

		hasher: sha256.New(),
		f:      f,

		buf:    make([]byte, batchBufferSize),
		writer: bufio.NewWriterSize(f, a.bufferSize),
	}
	// If we don't need to recycle, we should create a new hashmap for this
	// batch.
	recycled, err := b.recycle()
	if err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return nil, err
	}
	if !recycled {
		b.alive, _ = a.preallocAlive.PopLeft()
	}
	if b.alive == nil {
		panic("empty alive")
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
func (b *Batch) recycle() (bool, error) {
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
	b.alive = b.a.batches[oldestBatch].alive

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
		b.alive.Put(k, b.put(k, value))
		b.recycled++
	}
	b.pruneableBatch = &oldestBatch
	return true, nil
}

// TODO: make this a true abort as long as called before Prepare
//
// We must release this lock to shutdown properly
func (b *Batch) Abort() {
	// Delete in-progress file
	_ = b.f.Close()
	_ = os.Remove(b.path)

	// Release lock held acquired during [recycle]
	b.a.commitLock.Unlock()
}

func (b *Batch) put(key string, value []byte) *record {
	// Check input
	if b.err != nil {
		return nil
	}
	if len(key) > int(consts.MaxUint16) {
		b.err = ErrKeyTooLong
		return nil
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
		b.err = err
		return nil
	}
	if _, err := b.hasher.Write(b.buf); err != nil {
		b.err = err
		return nil
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
	return r
}

// Prepare should be called write before we begin writing to the batch. As soon as
// this function is called, all reads to the database will be blocked.
func (b *Batch) Prepare() int {
	b.a.keyLock.Lock()

	// Iterate over [alive] and update records for [keys] that were recycled
	aliveIter := b.alive.NewIterator()
	for aliveIter.Next() {
		key, record := aliveIter.Key(), aliveIter.Value()

		// Any past value stored in keys must be the [key] stored in [alive], so
		// we don't need to worry about deleting old values from alive.
		b.a.keys[key] = record
	}
	return b.recycled
}

func (b *Batch) Put(_ context.Context, key string, value []byte) error {
	if b.err != nil {
		return b.err
	}

	record := b.put(key, value)
	if record == nil {
		// An error occurred
		return b.err
	}
	b.alive.Put(key, record)

	// Cleanup old record
	past, ok := b.a.keys[key]
	if ok {
		// Delete old items from linked hashmap
		//
		// We check to see if the past batch is less
		// than the current batch because we may have just recycled
		// this key and it is already in [alive].
		if past.batch < b.batch {
			b.a.batches[past.batch].alive.Delete(key)
		}

		// Use existing value instead of inserting a new value
		//
		// This value was also used in [alive], so we don't need
		// to reclaim that too.
		past.batch = record.batch
		past.value = record.value
		past.loc = record.loc
		past.size = record.size
	} else {
		b.a.keys[key] = record
	}
	return nil
}

func (b *Batch) Delete(_ context.Context, key string) error {
	if b.err != nil {
		return b.err
	}

	// Check input
	if len(key) > int(consts.MaxUint16) {
		return ErrKeyTooLong
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

	// Cleanup any old keys
	past, ok := b.a.keys[key]
	delete(b.a.keys, key)
	if ok {
		// Delete old items from linked hashmap
		//
		// We check to see if the past batch is less
		// than the current batch because we may have just recycled
		// this key and it is already in [alive].
		if past.batch < b.batch {
			b.a.batches[past.batch].alive.Delete(key)
		} else {
			b.alive.Delete(key)
		}
	}
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

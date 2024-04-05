package appenddb

import (
	"bufio"
	"bytes"
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
	"github.com/ava-labs/hypersdk/consts"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"
	"golang.org/x/exp/mmap"
)

const (
	opPut    = uint8(0)
	opDelete = uint8(1)

	defaultBatchBufferSize = 16_384 // per item encoding
	minDiskValueSize       = 512
)

type record struct {
	batch uint64

	value []byte

	// Location of value in file (does not include
	// operation, key length, key, or value length)
	loc  int64
	size uint32
}

type tracker struct {
	reader *mmap.ReaderAt
	alive  *linked.Hashmap[string, *record]
}

// This sits under the MerkleDB and won't be used
// directly by the VM.
type AppendDB struct {
	logger     logging.Logger
	baseDir    string
	bufferSize int
	historyLen uint64

	commitLock  sync.RWMutex
	oldestBatch *uint64
	nextBatch   uint64
	size        uint64

	keyLock       sync.RWMutex
	leftoverAlive *linked.Hashmap[string, *record] // avoid reallocation
	batches       map[uint64]*tracker
	keys          map[string]*record
}

func readRecord(batch uint64, reader io.Reader, cursor int64, hasher hash.Hash) (string, *record, int64, error) {
	// Read op and key len
	opAndKeyLen := make([]byte, 1+consts.Uint16Len)
	if _, err := io.ReadFull(reader, opAndKeyLen); err != nil {
		return "", nil, -1, err
	}
	if hasher != nil {
		if _, err := hasher.Write(opAndKeyLen); err != nil {
			return "", nil, -1, err
		}
	}
	cursor += int64(len(opAndKeyLen))
	op := opAndKeyLen[0]
	keyLen := binary.BigEndian.Uint16(opAndKeyLen[1:])

	switch op {
	case opPut:
		// Read key and value len
		keyAndValueLen := make([]byte, keyLen+consts.Uint32Len)
		if _, err := io.ReadFull(reader, keyAndValueLen); err != nil {
			return "", nil, -1, err
		}
		if hasher != nil {
			if _, err := hasher.Write(keyAndValueLen); err != nil {
				return "", nil, -1, err
			}
		}
		cursor += int64(len(keyAndValueLen))
		key := string(keyAndValueLen[:keyLen])
		valueLen := binary.BigEndian.Uint32(keyAndValueLen[keyLen:])

		// Read value
		valueStart := cursor
		value := make([]byte, valueLen)
		if _, err := io.ReadFull(reader, value); err != nil {
			return "", nil, -1, err
		}
		if hasher != nil {
			if _, err := hasher.Write(value); err != nil {
				return "", nil, -1, err
			}
		}
		cursor += int64(len(value))
		r := &record{batch: batch}
		if valueLen >= minDiskValueSize {
			r.loc = valueStart
			r.size = valueLen
		} else {
			r.value = value
			r.loc = -1
		}
		return key, r, cursor, nil
	case opDelete:
		// Read key
		key := make([]byte, keyLen)
		if _, err := io.ReadFull(reader, key); err != nil {
			return "", nil, -1, err
		}
		if hasher != nil {
			if _, err := hasher.Write(key); err != nil {
				return "", nil, -1, err
			}
		}
		cursor += int64(len(key))
		return string(key), nil, cursor, nil
	default:
		return "", nil, -1, fmt.Errorf("%w: invalid operation %d", ErrCorrupt, op)
	}
}

func loadBatch(path string, file uint64) (ids.ID, *linked.Hashmap[string, *record], error) {
	// Load file into buffer
	f, err := os.Open(path)
	if err != nil {
		return ids.Empty, nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return ids.Empty, nil, err
	}
	reader := bufio.NewReader(f)

	// Read all operations from the file
	var (
		alive    = linked.NewHashmap[string, *record]()
		fileSize = int64(fi.Size())
		cursor   = int64(0)
		hasher   = sha256.New()
	)
	for cursor < fileSize-ids.IDLen {
		key, entry, newCursor, err := readRecord(file, reader, cursor, hasher)
		if err != nil {
			return ids.Empty, nil, err
		}
		alive.Put(key, entry)
		cursor = newCursor
	}
	checksum := make([]byte, sha256.Size)
	if _, err := io.ReadFull(reader, checksum); err != nil {
		return ids.Empty, nil, err
	}
	if !bytes.Equal(checksum, hasher.Sum(nil)) {
		return ids.Empty, nil, fmt.Errorf("%w: checksum mismatch", ErrCorrupt)
	}
	if cursor+sha256.Size != fileSize {
		return ids.Empty, nil, fmt.Errorf("%w: incorrect size", ErrCorrupt)
	}
	return ids.ID(checksum), alive, nil
}

// New returns a new AppendDB instance and the ID of the last committed file.
func New(
	log logging.Logger,
	baseDir string,
	initialSize int,
	bufferSize int,
	historyLen uint64, // should not be changed
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
		checksum, alive, err := loadBatch(path, file)
		if err != nil {
			if errors.Is(err, ErrCorrupt) {
				log.Warn("detected corrupt batch", zap.String("path", path), zap.Error(err))
				for j := i; j < len(files); j++ {
					corruptPath := filepath.Join(baseDir, strconv.FormatUint(files[j], 10))
					_ = os.Remove(corruptPath)
					log.Warn("removed corrupt batch", zap.String("path", corruptPath))
				}
				break
			}
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

func (a *AppendDB) Get(key string) ([]byte, error) {
	a.keyLock.RLock()
	defer a.keyLock.RUnlock()

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

	deadRecords buffer.Deque[*record]
	buf         []byte
	writer      *bufio.Writer
	alive       *linked.Hashmap[string, *record]

	err error
}

func (a *AppendDB) NewBatch(changes int) (*Batch, error) {
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

		deadRecords: buffer.NewUnboundedDeque[*record](max(changes/2, 16_384)),
		buf:         make([]byte, defaultBatchBufferSize),
		writer:      bufio.NewWriterSize(f, a.bufferSize),
	}
	if a.leftoverAlive == nil {
		b.alive = linked.NewHashmapWithSize[string, *record](changes)
	} else {
		b.alive = a.leftoverAlive
		a.leftoverAlive = nil
		b.alive.Clear()
	}
	b.recycle()
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
func (b *Batch) recycle() {
	// Determine if we should delete the oldest batch
	if b.batch < b.a.historyLen {
		return
	}
	if b.a.oldestBatch == nil {
		return
	}
	oldestBatch := *b.a.oldestBatch
	if b.batch-b.a.historyLen <= oldestBatch {
		// This means that if we are working on batch 2, we will prune batch 0
		// but keep batch 1 around.
		return
	}

	// Collect all alive records and add them to the batch file
	aliveIter := b.a.batches[oldestBatch].alive.NewIterator()
	for aliveIter.Next() {
		k, record := aliveIter.Key(), aliveIter.Value()

		// Do nothing if the record was a delete (state would already have this accounted
		// for on restart)
		if record == nil {
			continue
		}

		// Anything left in [alive] is should be written, so we don't
		// need to check the batch in key.
		value, err := b.a.Get(k)
		if err != nil {
			b.err = err
			return
		}
		b.alive.Put(k, b.put(k, value))
		b.recycled++
	}
	b.pruneableBatch = &oldestBatch
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
	var r *record
	if rec, ok := b.deadRecords.PopRight(); ok {
		r = rec
	} else {
		r = &record{}
	}
	r.batch = b.batch
	lv := len(value)
	if lv >= minDiskValueSize {
		r.value = nil
		r.loc = valueStart
		r.size = uint32(lv)
	} else {
		r.value = value
		r.loc = -1
		r.size = 0
	}
	return r
}

// Prepare should be called write before we begin writing to the batch. As soon as
// this function is called, all reads to the database will be blocked.
func (b *Batch) Prepare() int {
	b.a.keyLock.Lock()

	// Iterate over [alive] and update [keys] that were recycled
	aliveIter := b.alive.NewIterator()
	for aliveIter.Next() {
		key, record := aliveIter.Key(), aliveIter.Value()
		past, ok := b.a.keys[key]
		if ok {
			// Delete old items from linked hashmap
			b.a.batches[past.batch].alive.Delete(key)

			// Use existing value instead of inserting a new value
			//
			// This value was also used in [alive], so we don't need
			// to reclaim that too.
			past.batch = record.batch
			past.value = record.value
			past.loc = record.loc
			past.size = record.size
			b.deadRecords.PushRight(record)
		} else {
			b.a.keys[key] = record
		}
	}
	return b.recycled
}

func (b *Batch) Put(key string, value []byte) {
	record := b.put(key, value)
	if record == nil {
		// An error occurred
		return
	}
	b.alive.Put(key, record)

	// Cleanup old record
	past, ok := b.a.keys[key]
	if ok {
		// Delete old items from linked hashmap
		b.a.batches[past.batch].alive.Delete(key)

		// Use existing value instead of inserting a new value
		//
		// This value was also used in [alive], so we don't need
		// to reclaim that too.
		past.batch = record.batch
		past.value = record.value
		past.loc = record.loc
		past.size = record.size
		b.deadRecords.PushRight(record)
	} else {
		b.a.keys[key] = record
	}
}

func (b *Batch) Delete(key string) {
	// Check input
	if b.err != nil {
		return
	}
	if len(key) > int(consts.MaxUint16) {
		b.err = ErrKeyTooLong
		return
	}

	// Create operation to write
	l := consts.Uint8Len + consts.Uint16Len + len(key)
	b.growBuffer(l)
	b.buf[0] = opDelete
	binary.BigEndian.PutUint16(b.buf[1:], uint16(len(key)))
	copy(b.buf[1+consts.Uint16Len:], key)

	// Write to disk
	if _, err := b.writer.Write(b.buf); err != nil {
		b.err = err
		return
	}
	if _, err := b.hasher.Write(b.buf); err != nil {
		b.err = err
		return
	}
	b.cursor += int64(l)

	// Cleanup any old keys
	past, ok := b.a.keys[key]
	delete(b.a.keys, key)
	if ok {
		b.a.batches[past.batch].alive.Delete(key)
		b.deadRecords.PushRight(past)
	}
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
		b.a.leftoverAlive = preparedReader.alive
	}
	if b.a.oldestBatch == nil {
		b.a.oldestBatch = &b.batch
	}
	return ids.ID(checksum), nil
}

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
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
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
)

type record struct {
	// Location of value in file (does not include
	// operation, key length, key, or value length)
	batch uint64
	loc   int64
	size  uint32
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

	keyLock sync.RWMutex
	batches map[uint64]*tracker
	keys    map[string]*record
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
		return key, &record{
			batch: batch,
			loc:   valueStart,
			size:  valueLen,
		}, cursor, nil
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
			if entry == nil {
				delete(keys, key)
				alive.Delete(key) // only track keys that are alive
			} else {
				keys[key] = entry
			}
			if ok {
				batches[past.batch].alive.Delete(key)
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
type Batch struct {
	a     *AppendDB
	path  string
	batch uint64

	prepared       atomic.Bool
	pruneableBatch *uint64

	hasher hash.Hash
	f      *os.File
	cursor int64

	buf    []byte
	writer *bufio.Writer
	alive  *linked.Hashmap[string, *record]

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
	return &Batch{
		a:     a,
		path:  path,
		batch: batch,

		hasher: sha256.New(),
		f:      f,

		buf:    make([]byte, defaultBatchBufferSize),
		writer: bufio.NewWriterSize(f, a.bufferSize),
		alive:  linked.NewHashmapWithSize[string, *record](changes),
	}, nil
}

func (b *Batch) growBuffer(size int) {
	if cap(b.buf) < size {
		b.buf = make([]byte, size, size*2)
	} else {
		b.buf = b.buf[:size]
	}
}

// Prepare reads all keys from the oldest batch
// and puts them into the new file.
//
// A byproduct of this approach is that batches may contain duplicate
// keys (if a key is replaced in this batch), however, we are willing to make
// that tradeoff to begin writing the new batch as soon as possible.
//
// It is safe to call prepare multiple times.
func (b *Batch) Prepare() int {
	defer b.prepared.Store(true)

	if b.prepared.Load() {
		panic("batch already prepared")
	}

	// Determine if we should delete the oldest batch
	if b.batch < b.a.historyLen {
		return 0
	}
	if b.a.oldestBatch == nil {
		return 0
	}
	oldestBatch := *b.a.oldestBatch
	if b.batch-b.a.historyLen < oldestBatch {
		return 0
	}

	// Collect all alive records and add them to the batch file
	recycled := 0
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
			return 0
		}
		b.Put(k, value)
		recycled++
	}
	b.pruneableBatch = &oldestBatch
	return recycled
}

func (b *Batch) Put(key string, value []byte) {
	// Check input
	if b.err != nil {
		return
	}
	if len(key) > int(consts.MaxUint16) {
		b.err = ErrKeyTooLong
		return
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
		return
	}
	if _, err := b.hasher.Write(b.buf); err != nil {
		b.err = err
		return
	}

	// Add to changes
	b.alive.Put(key, &record{
		batch: b.batch,
		loc:   valueStart,
		size:  uint32(len(value)),
	})
	b.cursor += int64(l)
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

	// Add to changes
	b.alive.Put(key, nil)
	b.cursor += int64(l)
}

// ungracefulWrite cleans up any partial file writes
// in the case of an error and resets the database file count.
func (b *Batch) ungracefulWrite() {
	b.a.nextBatch--
	_ = b.f.Close()
	_ = os.Remove(b.path)
	return
}

// Write fsyncs the changes to disk and opens
// the file for reading.
//
// It then begins a cleanup routine to remove old
// files (to prevent the disk from filling up).
func (b *Batch) Write() (ids.ID, error) {
	defer b.a.commitLock.Unlock()

	// Check if we properly prepared this batch
	if !b.prepared.Load() && b.err == nil {
		b.err = ErrNotPrepared
	}

	// If we already encountered an error, return that immediately
	if b.err != nil {
		b.ungracefulWrite()
		return ids.Empty, b.err
	}

	// Add checksum to file (allows for crash recovery on restart in the case that a file is partially written)
	checksum := b.hasher.Sum(nil)
	if _, err := b.writer.Write(checksum); err != nil {
		b.ungracefulWrite()
		return ids.Empty, err
	}

	// Flush all unwritten data to disk
	if err := b.writer.Flush(); err != nil {
		b.ungracefulWrite()
		return ids.Empty, err
	}

	// Close file now that we don't need to write to it anymore
	//
	// Note: we don't require the file to be fsync'd here and assume
	// we can recover the current state on restart.
	if err := b.f.Close(); err != nil {
		b.ungracefulWrite()
		return ids.Empty, err
	}

	// Open file for mmap before keys become acessible
	reader, err := mmap.Open(b.path)
	if err != nil {
		// Should never happen
		return ids.Empty, err
	}
	b.a.size += uint64(reader.Len())

	// Update in-memory values with new locations
	//
	// We wait to do this until now to avoid locking all reads
	// while we update the keys.
	start := time.Now()
	b.a.keyLock.Lock()
	b.a.batches[b.batch] = &tracker{reader: reader, alive: b.alive}
	aliveIter := b.alive.NewIterator()
	for aliveIter.Next() {
		key, entry := aliveIter.Key(), aliveIter.Value()
		past, ok := b.a.keys[key]
		if entry == nil {
			delete(b.a.keys, key)
			b.alive.Delete(key) // only track keys that are alive
		} else {
			b.a.keys[key] = entry
		}
		if ok {
			b.a.batches[past.batch].alive.Delete(key)
		}
	}
	b.a.keyLock.Unlock()
	b.a.logger.Info("update keys", zap.Duration("duration", time.Since(start)))

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

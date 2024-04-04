package appenddb

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"hash"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/consts"
	"go.uber.org/zap"
	"golang.org/x/exp/mmap"
)

const (
	opPut    = uint8(0)
	opDelete = uint8(1)
)

type record struct {
	// Location of value in file (does not include
	// operation, key length, key, or value length)
	file uint64
	loc  int64
	size uint32
}

// This sits under the MerkleDB and won't be used
// directly by the VM.
type AppendDB struct {
	logger  logging.Logger
	baseDir string
	history uint64

	commitLock  sync.RWMutex
	nextBatch   uint64
	oldestBatch *uint64
	size        uint64

	keyLock sync.RWMutex
	batches map[uint64]*mmap.ReaderAt
	keys    map[string]*record
}

func readKey(file uint64, reader *mmap.ReaderAt, cursor int64, hasher hash.Hash) (string, *record, int64, error) {
	// Read op and key len
	opAndKeyLen := make([]byte, 1+consts.Uint16Len)
	if _, err := reader.ReadAt(opAndKeyLen, cursor); err != nil {
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
		if _, err := reader.ReadAt(keyAndValueLen, cursor); err != nil {
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
		if _, err := reader.ReadAt(value, cursor); err != nil {
			return "", nil, -1, err
		}
		if hasher != nil {
			if _, err := hasher.Write(value); err != nil {
				return "", nil, -1, err
			}
		}
		cursor += int64(len(value))
		return key, &record{
			file: file,
			loc:  valueStart,
			size: valueLen,
		}, cursor, nil
	case opDelete:
		// Read key
		key := make([]byte, keyLen)
		if _, err := reader.ReadAt(key, cursor); err != nil {
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
		return "", nil, -1, ErrCorrupt
	}
}

func openBatch(path string, file uint64) (ids.ID, map[string]*record, *mmap.ReaderAt, error) {
	// MMap file on-disk
	reader, err := mmap.Open(path)
	if err != nil {
		return ids.Empty, nil, nil, err
	}

	// Read all operations from the file
	var (
		changes  = make(map[string]*record)
		fileSize = int64(reader.Len())
		cursor   = int64(0)
		hasher   = sha256.New()
	)
	for cursor < fileSize-ids.IDLen {
		key, entry, newCursor, err := readKey(file, reader, cursor, hasher)
		if err != nil {
			return ids.Empty, nil, reader, err
		}
		changes[key] = entry
		cursor = newCursor
	}
	checksum := make([]byte, sha256.Size)
	if _, err := reader.ReadAt(checksum, cursor); err != nil {
		return ids.Empty, nil, reader, err
	}
	if !bytes.Equal(checksum, hasher.Sum(nil)) {
		return ids.Empty, nil, reader, ErrCorrupt
	}
	if cursor+sha256.Size != fileSize {
		return ids.Empty, nil, reader, ErrCorrupt
	}
	return ids.ID(checksum), changes, reader, nil
}

// New returns a new AppendDB instance and the ID of the last committed file.
func New(
	log logging.Logger,
	baseDir string,
	history uint64, // should not be changed
) (*AppendDB, ids.ID, error) {
	// Iterate over files in directory and put into sorted order
	files := []uint64{}
	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
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
	log.Info("found batches", zap.Uint64("count", uint64(len(files))))

	// Replay all changes on-disk
	//
	// Note: this will require reading all stored files
	// to reconstruct data. If any files were not fully written or have become corrupt,
	// this will error.
	var (
		keys    = make(map[string]*record)
		batches = make(map[uint64]*mmap.ReaderAt)
		size    uint64

		lastChecksum ids.ID
		firstBatch   uint64
		lastBatch    uint64
	)
	slices.Sort(files)
	for i, file := range files {
		path := filepath.Join(baseDir, strconv.FormatUint(file, 10))
		checksum, changes, reader, err := openBatch(path, file)
		if err != nil {
			_ = reader.Close()
			if errors.Is(err, ErrCorrupt) {
				log.Warn("found corrupt batch", zap.Uint64("file", file))
				for j := i; j < len(files); j++ {
					corruptPath := filepath.Join(baseDir, strconv.FormatUint(files[j], 10))
					_ = os.Remove(corruptPath)
					log.Warn("removed corrupt batch", zap.String("path", corruptPath))
				}
			}
			break
		}
		for key, entry := range changes {
			if entry == nil {
				delete(keys, key)
			} else {
				keys[key] = entry
			}
		}
		if i == 0 {
			firstBatch = file
		}
		batches[file] = reader
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
		zap.Uint64("size", size),
	)
	adb := &AppendDB{
		logger:  log,
		baseDir: baseDir,
		history: history,
	}
	if len(batches) > 0 {
		adb.oldestBatch = &firstBatch
		adb.nextBatch = lastBatch + 1
		adb.size = size
		adb.batches = batches
		adb.keys = keys
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
	_, err := a.batches[entry.file].ReadAt(value, entry.loc)
	return value, err
}

func (a *AppendDB) Close() error {
	for _, file := range a.batches {
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

func (a *AppendDB) readOldest() {
	var (
		batch     = *a.oldestBatch
		reader    = a.batches[batch]
		batchSize = int64(reader.Len())
		cursor    = int64(0)
		alive     = make(map[string]*record)
	)
	for cursor < batchSize-ids.IDLen {
		// Read next record
		key, record, newCursor, err := readKey(batch, reader, cursor, nil)
		if err != nil {
			log.Fatal("could not read key from batch", zap.Error(err))
			return
		}
		cursor = newCursor

		// Update alive if record hasn't been updated
		if record == nil {
			continue
		}
		a.keyLock.RLock()
		v, ok := a.keys[key]
		a.keyLock.RLock()
		if !ok || v.file > batch {
			continue
		}
		alive[key] = record
	}
}

// Batch is not thread-safe
type Batch struct {
	a    *AppendDB
	path string
	id   uint64

	hasher  hash.Hash
	f       *os.File
	changes map[string]*record
	cursor  int64

	err error
}

func (a *AppendDB) NewBatch(changes int) (*Batch, error) {
	a.commitLock.Lock()
	id := a.nextBatch
	path := filepath.Join(a.baseDir, strconv.FormatUint(id, 10))
	f, err := os.Create(path)
	if err != nil {
		a.commitLock.Unlock()
		return nil, err
	}
	a.nextBatch++
	return &Batch{
		a:    a,
		path: path,
		id:   id,

		hasher:  sha256.New(),
		f:       f,
		changes: make(map[string]*record, changes),
	}, nil
}

func (b *Batch) Put(key string, value []byte) {
	if b.err != nil {
		return
	}
	if len(key) > int(consts.MaxUint16) {
		b.err = ErrKeyTooLong
		return
	}
	if _, ok := b.changes[key]; ok {
		b.err = ErrDuplicate
		return
	}
	valueStart := b.cursor + int64(consts.Uint8Len+consts.Uint16Len+len(key)+consts.Uint32Len)
	operation := make([]byte, consts.Uint8Len+consts.Uint16Len+len(key)+consts.Uint32Len+len(value))
	operation[0] = opPut
	binary.BigEndian.PutUint16(operation[1:1+consts.Uint16Len], uint16(len(key)))
	copy(operation[1+consts.Uint16Len:], key)
	binary.BigEndian.PutUint32(operation[1+consts.Uint16Len+len(key):], uint32(len(value)))
	copy(operation[1+consts.Uint16Len+len(key)+consts.Uint32Len:], value)
	if _, err := b.f.Write(operation); err != nil {
		b.err = err
		return
	}
	if _, err := b.hasher.Write(operation); err != nil {
		b.err = err
		return
	}
	b.changes[key] = &record{
		file: b.id,
		loc:  valueStart,
		size: uint32(len(value)),
	}
}

func (b *Batch) Delete(key string) {
	if b.err != nil {
		return
	}
	if _, ok := b.changes[key]; ok {
		b.err = ErrDuplicate
		return
	}
	operation := make([]byte, consts.Uint8Len+consts.Uint16Len+len(key))
	operation[0] = opDelete
	binary.BigEndian.PutUint16(operation[1:1+consts.Uint16Len], uint16(len(key)))
	copy(operation[1+consts.Uint16Len:], key)
	if _, err := b.f.Write(operation); err != nil {
		b.err = err
		return
	}
	if _, err := b.hasher.Write(operation); err != nil {
		b.err = err
		return
	}
	b.changes[key] = nil
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

	// If we already encountered an error, return that immediately
	if b.err != nil {
		b.ungracefulWrite()
		return ids.Empty, b.err
	}

	// Add checksum to file (allows for crash recovery on restart in the case that a file is partially written)
	checksum := b.hasher.Sum(nil)
	if _, err := b.f.Write(checksum); err != nil {
		b.ungracefulWrite()
		return ids.Empty, err
	}

	// Ensure file is committed to disk
	if err := b.f.Sync(); err != nil {
		b.ungracefulWrite()
		return ids.Empty, err
	}

	// Close file now that we don't need to write to it anymore
	if err := b.f.Close(); err != nil {
		b.ungracefulWrite()
		return ids.Empty, err
	}

	// Open file for mmap
	reader, err := mmap.Open(b.path)
	if err != nil {
		// Should never happen
		return ids.Empty, err
	}
	b.a.size += uint64(reader.Len())

	// Update in-memory values with locations
	b.a.keyLock.Lock()
	b.a.batches[b.id] = reader
	for key, entry := range b.changes {
		if entry == nil {
			delete(b.a.keys, key)
		} else {
			b.a.keys[key] = entry
		}
	}
	b.a.keyLock.Unlock()

	// TODO: spawn new cleanup thread to queue changes
	// prepare for next batch async
	return ids.ID(checksum), nil
}

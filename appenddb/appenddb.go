package appenddb

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/hypersdk/consts"
	"golang.org/x/exp/mmap"
)

const (
	opPut    = uint8(0)
	opDelete = uint8(1)
)

type keyEntry struct {
	// Location of value in file (does not include
	// operation, key length, key, or value length)
	file uint64
	loc  int64
	size int
}

type AppendDB struct {
	baseDir string
	history uint64

	commitLock sync.RWMutex
	nextFile   uint64
	oldestFile uint64

	keyLock sync.RWMutex
	files   map[uint64]*mmap.ReaderAt
	keys    map[string]*keyEntry
}

func (a *AppendDB) Get(key string) ([]byte, error) {
	a.keyLock.RLock()
	defer a.keyLock.RUnlock()

	entry, ok := a.keys[key]
	if !ok {
		return nil, database.ErrNotFound
	}
	value := make([]byte, entry.size)
	_, err := a.files[entry.file].ReadAt(value, entry.loc)
	return value, err
}

// Batch is not thread-safe
type Batch struct {
	a    *AppendDB
	path string
	id   uint64

	hasher  hash.Hash
	f       *os.File
	changes map[string]*keyEntry
	cursor  int64

	err error
}

// TODO: load from disk and recover anything broken

func (a *AppendDB) NewBatch(changes int) (*Batch, error) {
	a.commitLock.Lock()
	id := a.nextFile
	path := filepath.Join(a.baseDir, strconv.FormatUint(id, 10))
	f, err := os.Create(path)
	if err != nil {
		a.commitLock.Unlock()
		return nil, err
	}
	a.nextFile++
	return &Batch{
		a:    a,
		path: path,
		id:   id,

		hasher:  sha256.New(),
		f:       f,
		changes: make(map[string]*keyEntry, changes),
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
	b.changes[key] = &keyEntry{
		file: b.id,
		loc:  valueStart,
		size: len(value),
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
	b.a.keyLock.RLock()
	_, ok := b.a.keys[key]
	b.a.keyLock.RUnlock()
	if !ok {
		b.err = database.ErrNotFound
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
	b.a.nextFile--
	_ = b.f.Close()
	_ = os.Remove(b.path)
	return
}

// Write fsyncs the changes to disk and opens
// the file for reading.
//
// It then begins a cleanup routine to remove old
// files (to prevent the disk from filling up).
func (b *Batch) Write() error {
	defer b.a.commitLock.Unlock()

	// If we already encountered an error, return that immediately
	if b.err != nil {
		b.ungracefulWrite()
		return b.err
	}

	// Add checksum to file (allows for crash recovery on restart in the case that a file is partially written)
	checksum := b.hasher.Sum(nil)
	if _, err := b.f.Write(checksum); err != nil {
		b.ungracefulWrite()
		return err
	}

	// Ensure file is committed to disk
	if err := b.f.Sync(); err != nil {
		b.ungracefulWrite()
		return err
	}

	// Close file now that we don't need to write to it anymore
	if err := b.f.Close(); err != nil {
		b.ungracefulWrite()
		return err
	}

	// Open file for mmap
	reader, err := mmap.Open(b.path)
	if err != nil {
		// Should never happen
		return err
	}

	// Update in-memory values with locations
	b.a.keyLock.Lock()
	b.a.files[b.id] = reader
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
	return nil
}

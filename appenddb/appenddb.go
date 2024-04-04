package appenddb

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/smap"
	"golang.org/x/exp/mmap"
)

const (
	opPut    = uint8(0)
	opDelete = uint8(1)
)

type keyEntry struct {
	pending []byte

	// Location of value in file (does not include
	// operation, key length, key, or value length)
	file uint64
	loc  int64
	size int
}

type AppendDB struct {
	baseDir   string
	fileCount uint64

	files map[uint64]*mmap.ReaderAt
	l     sync.RWMutex
	keys  *smap.SMap[*keyEntry]
}

func (a *AppendDB) Get(key string) ([]byte, error) {
	a.l.RLock()
	defer a.l.RUnlock()

	entry, ok := a.keys.Get(key)
	if !ok {
		return nil, database.ErrNotFound
	}
	if len(entry.pending) > 0 {
		return entry.pending, nil
	}
	value := make([]byte, entry.size)
	_, err := a.files[entry.file].ReadAt(value, entry.loc)
	return value, err
}

// Batch is not thread-safe
type Batch struct {
	a  *AppendDB
	id uint64

	f       *os.File
	changes map[string]*keyEntry
	cursor  int64

	err error
}

func (a *AppendDB) NewBatch() (*Batch, error) {
	a.l.Lock()
	id := a.fileCount
	path := filepath.Join(a.baseDir, strconv.FormatUint(id, 10))
	f, err := os.Create(path)
	if err != nil {
		a.l.Unlock()
		return nil, err
	}
	a.fileCount++
	return &Batch{a: a, f: f, id: id}, nil
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
	_, err := b.f.Write(nil)
	if err != nil && b.err == nil {
		b.err = err
	}
}

// Write fsyncs the changes to disk and opens
// the file for reading.
//
// It then begins a cleanup routine to remove old
// files (to prevent the disk from filling up).
func (b *Batch) Write() error {
	defer b.a.l.Unlock()

	// Ensure file is committed to disk
	if err := b.f.Sync(); err != nil {
		return err
	}
	if err := b.f.Close(); err != nil {
		return err
	}

	// TODO: Update in-memory values with locations

	// TODO: spawn new cleanup thread to queue changes
	return nil
}

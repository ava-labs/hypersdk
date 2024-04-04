package appenddb

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/hypersdk/smap"
	"golang.org/x/exp/mmap"
)

type keyEntry struct {
	pending []byte

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

	f      *os.File
	cursor int64

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
	_, err := b.f.Write(nil)
	if err != nil && b.err == nil {
		b.err = err
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

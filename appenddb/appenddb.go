package appenddb

import (
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
	base      string
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

type Batch struct {
	a *AppendDB
}

func (a *AppendDB) NewBatch() *Batch {
	a.l.Lock()
	return &Batch{a: a}
}

// Write fsyncs the changes to disk and opens
// the file for reading.
//
// It then begins a cleanup routine to remove old
// files (to prevent the disk from filling up).
func (b *Batch) Write() error {
	defer b.a.l.Unlock()

	// TODO: spawn new cleanup thread to queue changes
	return nil
}

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

func (a *AppendDB) Lock() {
	a.l.Lock()
}

func (a *AppendDB) Unlock() {
	a.l.Unlock()
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

package filedb

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/lockmap"
	"github.com/ava-labs/hypersdk/utils"
)

type FileDB struct {
	baseDir string
	sync    bool

	lm *lockmap.Lockmap

	fileCache cache.Cacher[string, []byte]
}

func New(baseDir string, sync bool, directoryCache int, dataCache int) *FileDB {
	return &FileDB{
		baseDir:   baseDir,
		sync:      sync,
		lm:        lockmap.New(16), // concurrent locks
		fileCache: cache.NewSizedLRU[string, []byte](dataCache, func(key string, value []byte) int { return len(key) + len(value) }),
	}
}

func (f *FileDB) Put(key string, value []byte, cache bool) error {
	filePath := filepath.Join(f.baseDir, key)

	// Don't do anything if already in cache and bytes equal
	if cachedValue, exists := f.fileCache.Get(filePath); exists {
		if bytes.Equal(cachedValue, value) {
			return nil
		}
	}

	// Put in cache before writing to disk so readers can still access if there
	// is a write backlog.
	if cache {
		f.fileCache.Put(filePath, value)
	}

	// Store the value on disk
	f.lm.Lock(filePath)
	defer f.lm.Unlock(filePath)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("%w: unable to create file", err)
	}
	defer file.Close()

	vid := utils.ToID(value)
	_, err = file.Write(vid[:])
	if err != nil {
		return fmt.Errorf("%w: unable to write to file", err)
	}
	_, err = file.Write(value)
	if err != nil {
		return fmt.Errorf("%w: unable to write to file", err)
	}
	if f.sync {
		if err := file.Sync(); err != nil {
			return fmt.Errorf("%w: unable to sync file", err)
		}
	}
	return nil
}

func (f *FileDB) Get(key string, cache bool) ([]byte, error) {
	filePath := filepath.Join(f.baseDir, key)

	// Attempt to read from cache
	if value, exists := f.fileCache.Get(filePath); exists {
		return value, nil
	}

	// Attempt to read from disk
	f.lm.RLock(filePath)
	defer f.lm.RUnlock(filePath)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, database.ErrNotFound
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("%w: unable to stat file", err)
	}

	diskValue := make([]byte, stat.Size())
	if _, err = file.Read(diskValue); err != nil {
		return nil, fmt.Errorf("%w: unable to read from file", err)
	}
	if len(diskValue) < consts.IDLen {
		return nil, fmt.Errorf("%w: less than IDLen found=%d", ErrCorrupt, len(diskValue))
	}
	value := diskValue[consts.IDLen:]
	vid := utils.ToID(value)
	did := ids.ID(diskValue[:consts.IDLen])
	if vid != did {
		return nil, fmt.Errorf("%w: found=%s expected=%s", ErrCorrupt, vid, did)
	}
	if cache {
		f.fileCache.Put(filePath, value)
	}
	return value, nil
}

func (f *FileDB) Has(key string) (bool, error) {
	filePath := filepath.Join(f.baseDir, key)

	// Attempt to reach from cache
	if _, exists := f.fileCache.Get(filePath); exists {
		return true, nil
	}

	// Attempt to read from disk
	f.lm.RLock(filePath)
	defer f.lm.RUnlock(filePath)
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (f *FileDB) Remove(key string) error {
	filePath := filepath.Join(f.baseDir, key)
	f.lm.Lock(filePath)
	defer f.lm.Unlock(filePath)

	if err := os.Remove(filePath); err != nil {
		return err
	}
	f.fileCache.Evict(filePath)
	return nil
}

// Close doesn't do anything but is canonical for a database to provide.
func (f *FileDB) Close() error { return nil }

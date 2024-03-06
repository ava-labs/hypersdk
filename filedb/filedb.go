package filedb

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/database"
)

type holderLock struct {
	holders int
	lock    sync.RWMutex
}

type FileDB struct {
	baseDir string
	sync    bool

	usedFilesLock sync.Mutex
	usedFiles     map[string]*holderLock

	fileCache cache.Cacher[string, []byte]
}

func New(baseDir string, sync bool, directoryCache int, dataCache int) *FileDB {
	return &FileDB{
		baseDir:   baseDir,
		sync:      sync,
		usedFiles: make(map[string]*holderLock),
		fileCache: cache.NewSizedLRU[string, []byte](dataCache, func(key string, value []byte) int { return len(key) + len(value) }),
	}
}

func (f *FileDB) lockFile(path string, write bool) {
	f.usedFilesLock.Lock()
	hl, exists := f.usedFiles[path]
	if exists {
		hl.holders++
		f.usedFilesLock.Unlock()
		if write {
			hl.lock.Lock()
		} else {
			hl.lock.RLock()
		}
		return
	}
	hl = &holderLock{holders: 1}
	if write {
		hl.lock.Lock()
	} else {
		hl.lock.RLock()
	}
	f.usedFiles[path] = hl
	f.usedFilesLock.Unlock()
}

func (f *FileDB) releaseFile(path string, write bool) {
	f.usedFilesLock.Lock()
	hl := f.usedFiles[path]
	if hl.holders > 1 {
		hl.holders--
		if write {
			hl.lock.Unlock()
		} else {
			hl.lock.RUnlock()
		}
	} else {
		delete(f.usedFiles, path)
	}
	f.usedFilesLock.Unlock()
}

func (f *FileDB) Put(key string, value []byte) error {
	filePath := filepath.Join(f.baseDir, key)
	f.lockFile(filePath, true)
	defer f.releaseFile(filePath, true)

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("%w: unable to create file", err)
	}
	defer file.Close()

	_, err = file.Write(value)
	if err != nil {
		return fmt.Errorf("%w: unable to write to file", err)
	}
	if f.sync {
		if err := file.Sync(); err != nil {
			return fmt.Errorf("%w: unable to sync file", err)
		}
	}
	f.fileCache.Put(filePath, value)
	return nil
}

func (f *FileDB) Get(key string) ([]byte, error) {
	filePath := filepath.Join(f.baseDir, key)
	f.lockFile(filePath, false)
	defer f.releaseFile(filePath, false)

	if value, exists := f.fileCache.Get(filePath); exists {
		return value, nil
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, database.ErrNotFound
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("%w: unable to stat file", err)
	}

	value := make([]byte, stat.Size())
	if _, err = file.Read(value); err != nil {
		return nil, fmt.Errorf("%w: unable to read from file", err)
	}
	f.fileCache.Put(filePath, value)
	return value, nil
}

func (f *FileDB) Has(key string) (bool, error) {
	filePath := filepath.Join(f.baseDir, key)
	f.lockFile(filePath, false)
	defer f.releaseFile(filePath, false)

	if _, exists := f.fileCache.Get(filePath); exists {
		return true, nil
	}

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
	f.lockFile(filePath, true)
	defer f.releaseFile(filePath, true)

	if err := os.Remove(filePath); err != nil {
		return err
	}
	f.fileCache.Evict(filePath)
	return nil
}

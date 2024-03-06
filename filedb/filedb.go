package filedb

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/database"
)

type FileDB struct {
	baseDir string
	sync    bool

	dirLock            sync.Mutex
	createdDirectories *cache.LRU[string, struct{}]

	fileLock sync.Mutex
	files    map[string]chan struct{}

	fileCache cache.Cacher[string, []byte]
}

func New(baseDir string, sync bool, directoryCache int, dataCache int) *FileDB {
	return &FileDB{
		baseDir:            baseDir,
		sync:               sync,
		createdDirectories: &cache.LRU[string, struct{}]{Size: directoryCache},
		files:              make(map[string]chan struct{}),
		fileCache:          cache.NewSizedLRU[string, []byte](dataCache, func(key string, value []byte) int { return len(key) + len(value) }),
	}
}

func (f *FileDB) createDirectories(directory string) error {
	f.dirLock.Lock()
	defer f.dirLock.Unlock()

	_, exists := f.createdDirectories.Get(directory)
	if !exists {
		if err := os.MkdirAll(filepath.Join(f.baseDir, directory), os.ModePerm); err != nil {
			return err
		}
		f.createdDirectories.Put(directory, struct{}{})
	}
	return nil
}

func (f *FileDB) lockFile(path string) {
	f.fileLock.Lock()
	waiter, exists := f.files[path]
	if !exists {
		f.files[path] = make(chan struct{})
		f.fileLock.Unlock()
		return
	}
	f.fileLock.Unlock()
	<-waiter
}

func (f *FileDB) releaseFile(path string) {
	f.fileLock.Lock()
	waiter := f.files[path]
	delete(f.files, path)
	f.fileLock.Unlock()

	close(waiter)
}

func (f *FileDB) Put(directory string, key string, value []byte) error {
	if err := f.createDirectories(directory); err != nil {
		return err
	}

	filePath := filepath.Join(f.baseDir, directory, key)
	f.lockFile(filePath)
	defer f.releaseFile(filePath)

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

func (f *FileDB) Get(directory string, key string) ([]byte, error) {
	filePath := filepath.Join(f.baseDir, directory, key)
	f.lockFile(filePath)
	defer f.releaseFile(filePath)

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

func (f *FileDB) Has(directory string, key string) (bool, error) {
	filePath := filepath.Join(f.baseDir, directory, key)
	f.lockFile(filePath)
	defer f.releaseFile(filePath)

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

// Remove removes the directory and all of its contents. It opts for speed
// rather than completeness and will not clear any caches.
//
// Remove can be called on the same directory multiple times (not concurrently).
//
// The caller should not read from or write to a directory that is being removed.
func (f *FileDB) Remove(directory string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(f.baseDir, directory))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	names := make([]string, len(entries))
	for i, entry := range entries {
		name := entry.Name()
		if err := os.Remove(filepath.Join(f.baseDir, directory, name)); err != nil {
			return nil, fmt.Errorf("%w: unable to remove file", err)
		}
		names[i] = name
	}
	return names, os.Remove(filepath.Join(f.baseDir, directory))
}

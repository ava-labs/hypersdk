// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fetcher

import (
	"context"
	"errors"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
)

// Fetcher retrieves values on-the-fly and ensures that
// a value is only fetched from disk once. Subsequent
// requests can be retrieved from cache.
type Fetcher struct {
	im        state.Immutable
	cacheLock sync.RWMutex
	cache     map[string]*fetchData

	keysToFetch map[string][]ids.ID        // Number of txns waiting for a key
	txnsToFetch map[ids.ID]*sync.WaitGroup // Number of keys a txn is waiting on
	keyLock     sync.RWMutex
	txnLock     sync.Mutex

	stopOnce  sync.Once
	stop      chan struct{}
	err       error
	fetchable chan *task
	l         sync.Mutex
	wg        sync.WaitGroup

	completed int
	numTxs    int
}

// Data to insert into the cache
type fetchData struct {
	v      []byte
	exists bool
	chunks uint16
}

// Holds the information that a worker needs to fetch values
type task struct {
	ctx context.Context
	id  ids.ID
	key string
	wg  *sync.WaitGroup
}

// New creates a new [Fetcher]
func New(numTxs int, concurrency int, im state.Immutable) *Fetcher {
	f := &Fetcher{
		im:          im,
		cache:       make(map[string]*fetchData, numTxs),
		keysToFetch: make(map[string][]ids.ID),
		txnsToFetch: make(map[ids.ID]*sync.WaitGroup, numTxs),
		fetchable:   make(chan *task),
		stop:        make(chan struct{}),
		numTxs:      numTxs,
	}
	for i := 0; i < concurrency; i++ {
		f.wg.Add(1)
		go f.runWorker()
	}
	return f
}

// Workers fetch individual keys
func (f *Fetcher) runWorker() {
	defer f.wg.Done()
	for {
		select {
		case t, ok := <-f.fetchable:
			if !ok {
				return
			}

			// Checks the cache twice. In isFirst, we will wait if it's not
			// in the cache and not the first tx requesting the key. We have
			// the second check to ensure when the tx is unblocked, it can
			// fetch from cache.
			f.isFirst(t)
			if exists := f.has(t.key); exists {
				f.updateDependencies(t.key)
				continue
			}

			// Fetch from disk once if key isn't already in cache
			v, err := f.im.GetValue(t.ctx, []byte(t.key))
			if errors.Is(err, database.ErrNotFound) {
				f.updateCache(t.key, nil, false, 0)
				f.updateDependencies(t.key)
				continue
			} else if err != nil {
				f.stopOnce.Do(func() {
					f.err = err
					close(f.stop)
				})
				return
			}

			// We verify that the [NumChunks] is already less than the number
			// added on the write path, so we don't need to do so again here.
			numChunks, ok := keys.NumChunks(v)
			if !ok {
				f.stopOnce.Do(func() {
					f.err = ErrInvalidKeyValue
					close(f.stop)
				})
				return
			}

			f.updateCache(t.key, v, true, numChunks)
			f.updateDependencies(t.key)
		case <-f.stop:
			return
		}
	}()
}

// Checks if a key is in the cache. Allows concurrent reads to cache.
func (f *Fetcher) has(k string) bool {
	f.cacheLock.RLock()
	defer f.cacheLock.RUnlock()
	_, exists := f.cache[k]
	return exists
}

// Checks if tx fetching this key was the first to request
func (f *Fetcher) isFirst(t *task) {
	f.keyLock.RLock()
	if exists := f.has(t.key); !exists && f.keysToFetch[t.key][0] != t.id {
		f.keyLock.RUnlock() // Release the lock before blocking
		t.wg.Wait()
	} else {
		f.keyLock.RUnlock()
	}
}

// Puts a key that was fetched from disk into cache
func (f *Fetcher) updateCache(k string, v []byte, exists bool, chunks uint16) {
	f.cacheLock.Lock()
	defer f.cacheLock.Unlock()
	f.cache[k] = &fetchData{v, exists, chunks}
}

// After fetching a key, decrement the tx count that was waiting on the key.
// This function is only called by the worker.
func (f *Fetcher) updateDependencies(k string) {
	f.keyLock.Lock()
	defer f.keyLock.Unlock()

	txIDs := f.keysToFetch[k]
	f.txnLock.Lock()
	for _, id := range txIDs {
		f.txnsToFetch[id].Done() // Notify all other txs
	}
	f.txnLock.Unlock()
	f.keysToFetch[k] = nil
}

// Lookup enqueues keys for the workers to fetch
func (f *Fetcher) Lookup(ctx context.Context, txID ids.ID, stateKeys state.Keys) *sync.WaitGroup {
	f.txnLock.Lock()
	wg := &sync.WaitGroup{}
	f.txnsToFetch[txID] = wg
	f.txnsToFetch[txID].Add(len(stateKeys))
	f.txnLock.Unlock()

	f.keyLock.Lock()
	tasks := make([]*task, 0, len(stateKeys))
	for k := range stateKeys {
		if _, ok := f.keysToFetch[k]; !ok {
			f.keysToFetch[k] = make([]ids.ID, 0)
		}
		f.keysToFetch[k] = append(f.keysToFetch[k], txID)
		t := &task{
			ctx: ctx,
			id:  txID,
			key: k,
			wg:  wg,
		}
		tasks = append(tasks, t)
	}
	f.keyLock.Unlock()

	for _, t := range tasks {
		f.fetchable <- t
	}
	return wg
}

func (f *Fetcher) Wait(wg *sync.WaitGroup, stateKeys state.Keys) (map[string]uint16, map[string][]byte) {
	// Block until all keys for the tx are fetched
	wg.Wait()

	f.l.Lock()
	f.completed++
	if f.completed == f.numTxs {
		close(f.fetchable)
	}
	f.l.Unlock()

	f.cacheLock.Lock()
	defer f.cacheLock.Unlock()

	// Fetch keys from cache
	var (
		reads   = make(map[string]uint16, len(stateKeys))
		storage = make(map[string][]byte, len(stateKeys))
	)
	for k := range stateKeys {
		if v, ok := f.cache[k]; ok {
			reads[k] = v.chunks
			if v.exists {
				storage[k] = v.v
			}
		}
	}
	return reads, storage
}

func (f *Fetcher) Stop() {
	f.stopOnce.Do(func() {
		f.err = ErrStopped
		close(f.fetchable)
		close(f.stop)
	})
}

// Wait until all the workers are done and return any errors
func (f *Fetcher) HandleErrors() error {
	f.wg.Wait()
	return f.err
}

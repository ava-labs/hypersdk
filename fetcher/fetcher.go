// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fetcher

import (
	"context"
	"errors"
	_ "fmt"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
)

// Fetcher retrieves values on-the-fly and
// ensures that a value is only fetched from
// disk once. Subsequent requests can be retrieved
// from cache.
type Fetcher struct {
	im        state.Immutable
	cacheLock sync.RWMutex
	cache     map[string]*fetchData

	keysToFetch map[string][]ids.ID        // Number of txns waiting for a key
	txnsToFetch map[ids.ID]*sync.WaitGroup // Number of keys a txn is waiting on

	// Lock for each map
	keysFetchLock sync.Mutex
	txnsFetchLock sync.Mutex

	stopOnce  sync.Once
	stop      chan struct{}
	err       error
	fetchable chan *task
	l         sync.Mutex

	completed int
	totalTxns int
}

// Data to insert into the cache
type fetchData struct {
	v      []byte
	exists bool

	chunks uint16
}

// task holds the information that a worker needs to fetch values
type task struct {
	ctx context.Context
	key string
}

// New creates a new [Fetcher]
func New(numTxs int, concurrency int, im state.Immutable) *Fetcher {
	f := &Fetcher{
		im:          im,
		cache:       make(map[string]*fetchData, numTxs),
		keysToFetch: make(map[string][]ids.ID),
		txnsToFetch: make(map[ids.ID]*sync.WaitGroup, numTxs),
		fetchable:   make(chan *task),
		stop:        make(chan struct{}), // TODO: implement Stop()
		totalTxns:   numTxs,
	}
	for i := 0; i < concurrency; i++ {
		f.createWorker()
	}
	return f
}

// Workers fetch individual keys
func (f *Fetcher) runWorker() {
	for {
		select {
		case t, ok := <-f.fetchable:
			if !ok {
				return
			}

			// Allow concurrent reads to cache
			if exists := f.isInCache(t.key); exists {
				continue
			}

			// Fetch from disk that aren't already in cache
			// We only ever fetch from disk once
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

// Checks if a key is in the cache
func (f *Fetcher) isInCache(k string) bool {
	f.cacheLock.RLock()
	defer f.cacheLock.RUnlock()

	inCache := false
	if _, ok := f.cache[k]; ok {
		inCache = true
		f.updateDependencies(k)
	}
	return inCache
}

// Writes a key that was fetched from disk into the cache
func (f *Fetcher) updateCache(k string, v []byte, exists bool, chunks uint16) {
	f.cacheLock.Lock()
	defer f.cacheLock.Unlock()
	f.cache[k] = &fetchData{v, exists, chunks}
}

// For a key that was fetched from disk or was already in cache, we decrement
// the count for the txID that was waiting on that key. This function is
// only called by the worker.
func (f *Fetcher) updateDependencies(k string) {
	f.keysFetchLock.Lock()
	defer f.keysFetchLock.Unlock()

	txIDs, _ := f.keysToFetch[k]
	f.txnsFetchLock.Lock()
	for _, id := range txIDs {
		f.txnsToFetch[id].Done()
	}
	f.txnsFetchLock.Unlock()

	// Clear queue of txns waiting for this key
	f.keysToFetch[k] = nil
}

// Lookup enqueues keys that we need to lookup to the workers, and
// returns a WaitGroup for a given transaction.
func (f *Fetcher) Lookup(ctx context.Context, txID ids.ID, stateKeys state.Keys) *sync.WaitGroup {
	f.txnsFetchLock.Lock()
	f.txnsToFetch[txID] = &sync.WaitGroup{}
	f.txnsToFetch[txID].Add(len(stateKeys))
	f.txnsFetchLock.Unlock()

	for k := range stateKeys {
		f.keysFetchLock.Lock()
		if _, ok := f.keysToFetch[k]; !ok {
			f.keysToFetch[k] = make([]ids.ID, 0)
		}
		f.keysToFetch[k] = append(f.keysToFetch[k], txID)
		t := &task{
			ctx: ctx,
			key: k,
		}
		// Release the lock to avoid deadlock when
		// calling updateDependencies
		f.keysFetchLock.Unlock()
		f.fetchable <- t
	}
	return f.txnsToFetch[txID]
}

// Block until worker finishes fetching keys and then fetch the
// keys from cache
func (f *Fetcher) Wait(wg *sync.WaitGroup, stateKeys state.Keys) (map[string]uint16, map[string][]byte) {
	wg.Wait()
	f.l.Lock()
	f.completed++
	if f.completed == f.totalTxns {
		close(f.fetchable)
	}
	f.l.Unlock()

	f.cacheLock.Lock()
	defer f.cacheLock.Unlock()

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

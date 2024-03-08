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
	im    state.Immutable
	cache map[string]*fetchData

	keysToFetch map[string]*keyData        // Number of txns waiting for a key
	txnsToFetch map[ids.ID]*sync.WaitGroup // Number of keys a txn is waiting on
	keyLock     sync.RWMutex
	txnLock     sync.Mutex

	stopOnce  sync.Once
	stop      chan struct{}
	err       error
	fetchable chan *task
	l         sync.RWMutex
	wg        sync.WaitGroup
	done      bool

	completed int
	numTxs    int
}

// Data to insert into the cache
type fetchData struct {
	v      []byte
	exists bool
	chunks uint16
}

// Data pertaining to whether a key was fetched or which txns are waiting
type keyData struct {
	fetched bool
	queue   []ids.ID
}

// Holds the information that a worker needs to fetch values
type task struct {
	ctx context.Context
	id  ids.ID
	key string
}

// New creates a new [Fetcher]
func New(numTxs int, concurrency int, im state.Immutable) *Fetcher {
	f := &Fetcher{
		im:          im,
		cache:       make(map[string]*fetchData, numTxs),
		keysToFetch: make(map[string]*keyData),
		txnsToFetch: make(map[ids.ID]*sync.WaitGroup, numTxs),
		fetchable:   make(chan *task, numTxs),
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

			// Allows concurrent reads to cache.
			if ok = f.shouldFetch(t); ok {
				continue
			}

			// Fetch from disk once if key isn't already in cache
			v, err := f.im.GetValue(t.ctx, []byte(t.key))
			if errors.Is(err, database.ErrNotFound) {
				f.update(t.key, nil, false, 0)
				continue
			} else if err != nil {
				f.stopOnce.Do(func() {
					f.err = err
					close(f.stop)
				})
				return
			}

			numChunks, ok := keys.NumChunks(v)
			if !ok {
				f.stopOnce.Do(func() {
					f.err = ErrInvalidKeyValue
					close(f.stop)
				})
				return
			}
			f.update(t.key, v, true, numChunks)
		case <-f.stop:
			return
		}
	}
}

// Checks if a key is in the cache. If it's not in cache, check
// if we were the first to request the key.
func (f *Fetcher) shouldFetch(t *task) bool {
	f.keyLock.RLock()
	defer f.keyLock.RUnlock()

	_, exists := f.cache[t.key]
	if !exists {
		queue := f.keysToFetch[t.key].queue
		if len(queue) > 0 && queue[0] != t.id {
			// Fetch other keys instead
			return true
		}
	}
	return exists
}

// Updates the cache and dependencies
func (f *Fetcher) update(k string, v []byte, exists bool, chunks uint16) {
	f.keyLock.Lock()
	defer f.keyLock.Unlock()

	// Puts a key that was fetched from disk into cache
	f.cache[k] = &fetchData{v, exists, chunks}

	f.txnLock.Lock()
	for _, id := range f.keysToFetch[k].queue {
		f.txnsToFetch[id].Done() // Notify all other txs
	}
	f.txnLock.Unlock()
	f.keysToFetch[k].queue = nil
	f.keysToFetch[k].fetched = true
}

// Lookup enqueues keys for the workers to fetch
func (f *Fetcher) Lookup(ctx context.Context, txID ids.ID, stateKeys state.Keys) *sync.WaitGroup {
	wg := &sync.WaitGroup{}
	f.txnLock.Lock()
	f.txnsToFetch[txID] = wg
	f.txnLock.Unlock()

	f.keyLock.Lock()
	tasks := make([]*task, 0, len(stateKeys))
	for k := range stateKeys {
		if _, ok := f.keysToFetch[k]; !ok {
			f.keysToFetch[k] = &keyData{fetched: false, queue: make([]ids.ID, 0)}
		}
		// Don't requeue keys that were already fetched
		if f.keysToFetch[k].fetched {
			continue
		}
		f.keysToFetch[k].queue = append(f.keysToFetch[k].queue, txID)
		t := &task{
			ctx: ctx,
			id:  txID,
			key: k,
		}
		tasks = append(tasks, t)
	}
	f.keyLock.Unlock()

	if done := f.stopped(); done {
		return wg // counter is zero
	}

	// Create wg based on number of keys we need to wait on
	wg.Add(len(tasks))
	for _, t := range tasks {
		f.fetchable <- t
	}
	return wg
}

func (f *Fetcher) Get(wg *sync.WaitGroup, stateKeys state.Keys) (map[string]uint16, map[string][]byte) {
	// Block until all keys for the tx are fetched
	wg.Wait()

	f.l.Lock()
	f.completed++
	if f.completed == f.numTxs {
		f.done = true
		close(f.fetchable)
	}
	f.l.Unlock()

	f.keyLock.Lock()
	defer f.keyLock.Unlock()

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

// Check if fetcher is stopped
func (f *Fetcher) stopped() bool {
	f.l.RLock()
	defer f.l.RUnlock()

	return f.done || f.err != nil
}

func (f *Fetcher) Stop() {
	f.stopOnce.Do(func() {
		f.err = ErrStopped
		f.done = true
		close(f.fetchable)
		close(f.stop)
	})
}

// Wait until all the workers are done and return any errors
func (f *Fetcher) Wait() error {
	f.wg.Wait()
	return f.err
}

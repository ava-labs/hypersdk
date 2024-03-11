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
	im state.Immutable

	l           sync.RWMutex
	keysToFetch map[string]*keyData        // Number of txns waiting for a key
	txnsToFetch map[ids.ID]*sync.WaitGroup // Number of keys a txn is waiting on

	stopOnce sync.Once
	stop     chan struct{}
	err      error

	wg        sync.WaitGroup
	waitOnce  sync.Once
	fetchable chan *task
}

// Data to insert into the cache
type fetchData struct {
	v      []byte
	exists bool
	chunks uint16
}

// Data pertaining to whether a key was fetched or which txns are waiting
type keyData struct {
	cache *fetchData
	queue []ids.ID
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
		keysToFetch: make(map[string]*keyData),
		txnsToFetch: make(map[ids.ID]*sync.WaitGroup, numTxs),
		fetchable:   make(chan *task, numTxs),
		stop:        make(chan struct{}),
	}
	f.wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
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

			if ok := f.shouldFetch(t); !ok {
				// Fetch another key
				continue
			}

			// Fetch from disk once
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
	f.l.RLock()
	defer f.l.RUnlock()

	if f.keysToFetch[t.key].cache != nil {
		return false
	}
	return f.keysToFetch[t.key].queue[0] == t.id
}

// Updates the cache and dependencies
func (f *Fetcher) update(k string, v []byte, exists bool, chunks uint16) {
	f.l.Lock()
	defer f.l.Unlock()

	// Puts a key that was fetched from disk into cache
	f.keysToFetch[k].cache = &fetchData{v, exists, chunks}
	for _, id := range f.keysToFetch[k].queue {
		f.txnsToFetch[id].Done() // Notify all other txs
	}
	f.keysToFetch[k].queue = nil
}

// Lookup enqueues keys for the workers to fetch
// Invariant: Don't call [Lookup] afer calling [Stop]
func (f *Fetcher) Lookup(ctx context.Context, txID ids.ID, stateKeys state.Keys) *sync.WaitGroup {
	f.l.Lock()
	tasks := make([]*task, 0, len(stateKeys))
	for k := range stateKeys {
		if _, ok := f.keysToFetch[k]; !ok {
			f.keysToFetch[k] = &keyData{cache: nil, queue: make([]ids.ID, 0)}
		}
		// Don't attempt to fetch keys that were already fetched
		if f.keysToFetch[k].cache != nil {
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
	wg := &sync.WaitGroup{}
	wg.Add(len(tasks))
	f.txnsToFetch[txID] = wg
	f.l.Unlock()

	for _, t := range tasks {
		f.fetchable <- t
	}
	return wg
}

func (f *Fetcher) Get(wg *sync.WaitGroup, stateKeys state.Keys) (map[string]uint16, map[string][]byte) {
	// Block until all keys for the tx are fetched
	wg.Wait()

	f.l.RLock()
	defer f.l.RUnlock()

	// Fetch keys from cache
	var (
		reads   = make(map[string]uint16, len(stateKeys))
		storage = make(map[string][]byte, len(stateKeys))
	)
	for k := range stateKeys {
		if v := f.keysToFetch[k].cache; v != nil {
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
		close(f.stop)
	})
}

// Wait until all the workers are done and return any errors.
//
// [Wait] can be called multiple times, however, [Lookup] should never be
// called after [Wait] is called.
func (f *Fetcher) Wait() error {
	f.waitOnce.Do(func() {
		close(f.fetchable)
	})
	f.wg.Wait()
	return f.err
}

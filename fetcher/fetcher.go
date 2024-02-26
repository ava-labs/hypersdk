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

// Fetcher retrieves values on-the-fly and
// ensures that a value is only fetched from
// disk once. Subsequent requests can be retrieved
// from cache.
type Fetcher struct {
	im        state.Immutable
	cacheLock sync.RWMutex
	cache     map[string]*FetchData

	keysToFetch map[string][]ids.ID // Number of txns waiting for a key
	txnsToFetch map[ids.ID]*sync.WaitGroup // Number of keys a txn is waiting on

	stopOnce  sync.Once
	stop      chan struct{}
	err       error
	fetchable chan *task
	l         sync.Mutex

	completed int
	totalTxns int
}

// Data to insert into the cache
type FetchData struct {
	Val    []byte
	Exists bool

	Chunks uint16
}

// task holds the information that a worker needs to fetch values
type task struct {
	ctx      context.Context
	id       ids.ID
	toLookup []string
}

// New creates a new [Fetcher]
func New(numTxs int, concurrency int, im state.Immutable) *Fetcher {
	f := &Fetcher{
		im:          im,
		cache:       make(map[string]*FetchData, numTxs),		
		txnsToFetch: make(map[ids.ID]*sync.WaitGroup, numTxs),
		fetchable:   make(chan *task),
		stop:        make(chan struct{}), // TODO: implement Stop()
		totalTxns:   numTxs,
	}
	for i := 0; i < concurrency; i++ {
		go f.runWorker()
	}
	return f
}

func (f *Fetcher) runWorker() {
	for {
		select {
		case t, ok := <-f.fetchable:
			if !ok {
				return
			}
			for _, k := range t.toLookup {
				// Allow concurrent reads to cache
				f.cacheLock.RLock()
				if _, ok := f.cache[k]; ok {
					f.txnsToFetch[t.id].Done()
					f.cacheLock.RUnlock()
					continue
				}
				f.cacheLock.RUnlock()

				// Fetch from disk that aren't already in cache
				// We only ever fetch from disk once
				v, err := f.im.GetValue(t.ctx, []byte(k))
				if errors.Is(err, database.ErrNotFound) {
					// Update the cache
					f.cacheLock.Lock()
					f.cache[k] = &FetchData{nil, false, 0}
					f.txnsToFetch[t.id].Done() // Decrement the count as we fetched one of the keys
					f.cacheLock.Unlock()
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

				f.cacheLock.Lock()
				f.cache[k] = &FetchData{v, true, numChunks}
				f.txnsToFetch[t.id].Done()
				f.cacheLock.Unlock()
			}

			f.l.Lock()
			f.completed++
			if f.completed == f.totalTxns {
				close(f.fetchable)
			}
			f.l.Unlock()
		case <-f.stop:
			return
		}
	}
}

// Lookup enqueues a set of stateKey values that we need to lookup, and
// returns a WaitGroup for a given transaction.
func (f *Fetcher) Lookup(ctx context.Context, txID ids.ID, stateKeys state.Keys) *sync.WaitGroup {
	// Get key names
	toLookup := make([]string, 0, len(stateKeys))
	for k := range stateKeys {
		toLookup = append(toLookup, k)
	}
	// Increment counter to number of keys to fetch
	f.txnsToFetch[txID] = &sync.WaitGroup{}
	f.txnsToFetch[txID].Add(len(toLookup))
	t := &task{
		ctx:      ctx,
		id:       txID,
		toLookup: toLookup,
	}
	// Go fetch from disk or verify it's in cache
	f.fetchable <- t
	return f.txnsToFetch[txID]
}

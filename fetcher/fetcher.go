// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fetcher

import (
	"context"
	"errors"
	"sync"
	_ "fmt"

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

	TxnsToFetch map[ids.ID]*sync.WaitGroup // Number of keys a txn is waiting on

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
	v    []byte
	exists bool

	chunks uint16
}

// task holds the information that a worker needs to fetch values
type task struct {
	ctx      context.Context
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

			// TODO: use break/continue?

			// Allow concurrent reads to cache
			if exists := f.isInCache(t.key); exists {
				//fmt.Printf("cache hit! %v\n", t.key)
			} else {
				//fmt.Printf("cache miss, fetching %v\n", t.key)
				// Fetch from disk that aren't already in cache
				// We only ever fetch from disk once
				v, err := f.im.GetValue(t.ctx, []byte(t.key))
				if errors.Is(err, database.ErrNotFound) {
					//fmt.Printf("cache miss %v\n", t.key)
					f.updateCache(t.key, nil, false, 0)
					//fmt.Printf("insert into cache %v\n", t.key)
					f.updateDependencies(t.key)
					//fmt.Printf("dep updated %v\n", t.key)
				} else if err != nil {
					f.stopOnce.Do(func() {
						f.err = err
						close(f.stop)
					})
					return
				} else {
					//fmt.Printf("fetching with value! %v\n", t.key)

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
				}
			}
		case <-f.stop:
			return
		}
	}()
}

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

func (f *Fetcher) updateCache(k string, v []byte, exists bool, chunks uint16) {
	f.cacheLock.Lock()
	defer f.cacheLock.Unlock()
	f.cache[k] = &fetchData{v, exists, chunks}
}

func (f *Fetcher) updateDependencies(k string) {
	//f.l.Lock()
	//defer f.l.Unlock()
	f.keysFetchLock.Lock()
	defer f.keysFetchLock.Unlock()

	// Don't need to check if k exists because this function 
	// is only called in the worker after we have already added
	// k to the map entry.
	//fmt.Printf("updateDependencies start\n")
	txIDs, _ := f.keysToFetch[k]
	//fmt.Printf("trying to acquire the lock\n")
	f.txnsFetchLock.Lock()
	//fmt.Printf("got in!\n")
	for _, id := range txIDs {
		// Decrement number of keys we're waiting on
		f.txnsToFetch[id].Done()
	}
	f.txnsFetchLock.Unlock()

	// Clear queue of txns waiting for this key and delete
	f.keysToFetch[k] = nil
	delete(f.keysToFetch, k)
	//fmt.Printf("updateDependencies finished\n")
}

// Lookup enqueues a set of stateKey values that we need to lookup, and
// returns a WaitGroup for a given transaction.
func (f *Fetcher) Lookup(ctx context.Context, txID ids.ID, stateKeys state.Keys) *sync.WaitGroup {
	//f.l.Lock()
	//defer f.l.Unlock()
	
	f.txnsFetchLock.Lock()
	//fmt.Printf("%v acquired the lock\n", txID)	
	f.txnsToFetch[txID] = &sync.WaitGroup{}
	f.txnsToFetch[txID].Add(len(stateKeys))
	//fmt.Printf("%v released the lock\n", txID)	
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
		f.keysFetchLock.Unlock()
		//fmt.Printf("sending into chan %v\n", k)
		f.fetchable <- t
		//fmt.Printf("out of chan %v\n", k)
	}

	// TODO: will this cause a panic if i don't hold the lock?
	return f.txnsToFetch[txID]
}

func (f *Fetcher) Wait(wg *sync.WaitGroup, stateKeys state.Keys) (map[string]uint16, map[string][]byte) {
	wg.Wait()
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

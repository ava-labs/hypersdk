// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fetcher

import (
	"context"
	"sync"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/keys"
)

type FetchData struct {
	Val      []byte
	Exists bool

	Chunks uint16
}

type Fetcher struct {
	im state.Immutable

	keysToFetch map[string]*sync.WaitGroup
	TxnsToFetch map[ids.ID]*sync.WaitGroup

	wg sync.WaitGroup
	stopOnce sync.Once
	stop     chan struct{}
	err error
	l sync.Mutex

	cacheLock sync.RWMutex
	Cache map[string]*FetchData

	fetchable chan *task
}

type task struct {
	ctx context.Context
	id ids.ID
	toLookup []string
}

func New(numTxs int, concurrency int, im state.Immutable) *Fetcher {
	f := &Fetcher {
		keysToFetch: make(map[string]*sync.WaitGroup),
		TxnsToFetch: make(map[ids.ID]*sync.WaitGroup, numTxs),
		fetchable: make(chan *task),
		im: im,
		Cache: make(map[string]*FetchData, numTxs),
	}
	for i := 0; i < concurrency; i++ {
		f.createWorker()
	}
	return f
}

func (f *Fetcher) createWorker() {
	f.wg.Add(1)

	go func() {
		defer f.wg.Done()

		for {
			select {
			case t, ok := <- f.fetchable:
				if !ok {
					return
				}

				// fetch from disk
				for _, k := range t.toLookup {
					/*f.cacheLock.RLock()
					if _, ok := f.Cache[k]; ok {
						fmt.Printf("nah\n")
						f.cacheLock.RUnlock()
						f.keysToFetch[k].Done()
						f.TxnsToFetch[t.id].Done()
						continue
					}
					f.cacheLock.RUnlock()*/

					v, err := f.im.GetValue(t.ctx, []byte(k))
					if errors.Is(err, database.ErrNotFound) {
						f.cacheLock.Lock()
						f.Cache[k] = &FetchData{nil, false, 0}
						f.cacheLock.Unlock()

						//f.keysToFetch[k].Done()
						f.TxnsToFetch[t.id].Done()
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
					f.Cache[k] = &FetchData{v, true, numChunks}
					f.cacheLock.Unlock()
					
					//f.keysToFetch[k].Done()
					f.TxnsToFetch[t.id].Done()
				}
			case <-f.stop:
				return	
			}
		}
	}()
}

func (f *Fetcher) Lookup(ctx context.Context, txID ids.ID, stateKeys state.Keys) {
	f.TxnsToFetch[txID] = &sync.WaitGroup{}
	toLookup := make([]string, 0, len(stateKeys))
	for k := range stateKeys {
		// find all keys that need to be fetched from disk
		f.cacheLock.Lock()
		if _, ok := f.Cache[k]; !ok {
			toLookup = append(toLookup, k)
			if _, ok := f.keysToFetch[k]; !ok {
				f.keysToFetch[k] = &sync.WaitGroup{}
			} else {
				f.keysToFetch[k].Add(1)
			}
		}
		f.cacheLock.Unlock()
	}
	if len(toLookup) > 0 {
		f.TxnsToFetch[txID].Add(len(toLookup))
		t := &task {
			ctx: ctx,
			id: txID,
			toLookup: toLookup,
		}
		f.fetchable <- t
	}
}


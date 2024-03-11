// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fetcher

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
)

// Fetcher retrieves values on-the-fly and ensures that
// a value is only fetched from data once. Subsequent
// requests can be retrieved from cache.
type Fetcher struct {
	im state.Immutable

	l    sync.RWMutex
	keys map[string]*key
	txs  map[ids.ID]*tx
	err  error

	wg       sync.WaitGroup
	tasks    chan *task
	waitOnce sync.Once

	stop     chan struct{}
	stopOnce sync.Once
}

type tx struct {
	wg   *sync.WaitGroup
	keys state.Keys
}

type task struct {
	ctx context.Context
	id  ids.ID
	key string
}

type key struct {
	cache   *data
	blocked []ids.ID
}

type data struct {
	v      []byte
	exists bool
	chunks uint16
}

// New creates a new [Fetcher]
func New(im state.Immutable, concurrency, txs int) *Fetcher {
	f := &Fetcher{
		im: im,

		keys: make(map[string]*key),
		txs:  make(map[ids.ID]*tx, txs),

		tasks: make(chan *task, txs),

		stop: make(chan struct{}),
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
		case t, ok := <-f.tasks:
			if !ok {
				return
			}

			v, err := f.im.GetValue(t.ctx, []byte(t.key))
			if errors.Is(err, database.ErrNotFound) {
				f.set(t.key, nil, false, 0)
				continue
			} else if err != nil {
				f.handleErr(err)
				return
			}
			numChunks, ok := keys.NumChunks(v)
			if !ok {
				f.handleErr(ErrInvalidKeyValue)
				return
			}
			f.set(t.key, v, true, numChunks)
		case <-f.stop:
			return
		}
	}
}

func (f *Fetcher) handleErr(err error) {
	f.stopOnce.Do(func() {
		f.l.Lock()
		f.err = err
		f.l.Unlock()
		close(f.stop)
	})
}

func (f *Fetcher) set(k string, v []byte, exists bool, chunks uint16) {
	f.l.Lock()
	defer f.l.Unlock()

	// Puts a key that was fetched from data into cache
	f.keys[k].cache = &data{v, exists, chunks}
	for _, id := range f.keys[k].blocked {
		f.txs[id].wg.Done() // Notify all other txs
	}
	f.keys[k].blocked = nil
}

// Fetch enqueues a set of [stateKeys] to be fetched from disk. Duplicate keys
// are only fetched once and fetch priority is done in the order [Fetch] is called.
//
// Fetch can be called concurrently.
//
// Invariant: Don't call [Fetch] afer calling [Stop] or [Wait]
func (f *Fetcher) Fetch(ctx context.Context, txID ids.ID, stateKeys state.Keys) error {
	f.l.Lock()
	if fErr := f.err; fErr != nil {
		f.l.Unlock()
		return fErr // ensures we don't deadlock if encountering an error
	}
	tasks := make([]*task, 0, len(stateKeys))
	for k := range stateKeys {
		d, ok := f.keys[k]
		if !ok {
			f.keys[k] = &key{blocked: []ids.ID{txID}}
			tasks = append(tasks, &task{
				ctx: ctx,
				id:  txID,
				key: k,
			})
			continue
		}

		// Don't attempt to fetch keys that were already fetched
		if d.cache != nil {
			continue
		}

		// Register to get notified when the key is fetched
		d.blocked = append(d.blocked, txID)
	}
	wg := &sync.WaitGroup{}
	wg.Add(len(tasks))
	f.txs[txID] = &tx{wg, stateKeys}
	f.l.Unlock()

	// Send fetch tasks to the workers
	for _, t := range tasks {
		f.tasks <- t
	}
	return nil
}

// Get will return the state keys fetched for the txID or return the error
// that caused fetching to fail.
//
// Get can be called concurrently.
func (f *Fetcher) Get(txID ids.ID) (map[string]uint16, map[string][]byte, error) {
	// Block until all keys for the tx are fetched or if the fetcher errored
	f.l.RLock()
	tx, ok := f.txs[txID]
	fErr := f.err // allows us to avoid deadlock when [tx.wg] will never be cleared
	f.l.RUnlock()
	if !ok {
		return nil, nil, fmt.Errorf("%w: unable to get keys for transaction %s", ErrMissingTx, txID)
	}
	if fErr != nil {
		return nil, nil, fErr
	}
	tx.wg.Wait()

	// Fetch keys from cache
	f.l.RLock()
	defer f.l.RUnlock()
	var (
		stateKeys = tx.keys
		reads     = make(map[string]uint16, len(stateKeys))
		storage   = make(map[string][]byte, len(stateKeys))
	)
	for k := range stateKeys {
		if v := f.keys[k].cache; v != nil {
			reads[k] = v.chunks
			if v.exists {
				storage[k] = v.v
			}
		}
	}
	return reads, storage, nil
}

// Stop terminates the fetcher before all keys have been fetched.
func (f *Fetcher) Stop() {
	f.handleErr(ErrStopped)
}

// Wait until all the workers are done and return any errors.
//
// [Wait] can be called multiple times, however, [Lookup] should never be
// called after [Wait] is called.
func (f *Fetcher) Wait() error {
	f.waitOnce.Do(func() {
		close(f.tasks)
	})
	f.wg.Wait()
	return f.err
}

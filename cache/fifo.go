// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cache

import (
	"sync"

	"github.com/ava-labs/avalanchego/utils/buffer"
)

type FIFO[K comparable, V any] struct {
	l sync.RWMutex

	buffer buffer.Queue[K]
	m      map[K]V
}

// NewFIFO creates a new First-In-First-Out cache of size [limit].
func NewFIFO[K comparable, V any](limit int) (*FIFO[K, V], error) {
	c := &FIFO[K, V]{
		m: make(map[K]V, limit),
	}
	buf, err := buffer.NewBoundedQueue(limit, c.remove)
	if err != nil {
		return nil, err
	}
	c.buffer = buf
	return c, nil
}

func (f *FIFO[K, V]) Put(key K, val V) {
	f.l.Lock()
	defer f.l.Unlock()

	f.buffer.Push(key) // Insert will remove the oldest [K] if we are at the [limit]
	f.m[key] = val
}

func (f *FIFO[K, V]) Get(key K) (V, bool) {
	f.l.RLock()
	defer f.l.RUnlock()

	v, ok := f.m[key]
	return v, ok
}

// remove is used as the callback in [BoundedBuffer]. It is assumed that the
// [WriteLock] is held when this is accessed.
func (f *FIFO[K, V]) remove(key K) {
	delete(f.m, key)
}

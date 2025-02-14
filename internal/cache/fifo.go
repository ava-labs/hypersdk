// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cache

import (
	"sync"

	"github.com/ava-labs/avalanchego/utils/buffer"
)

type FIFO[K comparable, V any] struct {
	l          sync.RWMutex
	buffer     buffer.Queue[K]
	m          map[K]V
	removedKey *K
}

// NewFIFO creates a new First-In-First-Out cache of size [limit].
//
// If a duplicate item is stored, it will not be requeued but its
// value will be changed.
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

// Put insert the [key, value] pair into the FIFO. In case an insert require the
// removal of an existing key due to size limits, the evicted key would get returned.
func (f *FIFO[K, V]) Put(key K, val V) (bool, *K) {
	f.l.Lock()
	defer f.l.Unlock()

	f.removedKey = nil
	var removedKey *K
	_, exists := f.m[key]
	if !exists {
		f.buffer.Push(key) // Push removes the oldest [K] if we are at the [limit]
		removedKey = f.removedKey
		f.removedKey = nil
	}

	f.m[key] = val
	return exists, removedKey
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
	f.removedKey = &key
}

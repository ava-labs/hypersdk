// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package emap

import (
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/heap"
)

// A LEmap implements en eviction map that stores the status
// of txs and their linked timestamps. The type [T] must implement the
// Item interface.
//
// A LEMap only allows the caller to check for the inclusion of some Item
// but not actually fetch that item.
//
// TODO: rename this/unify with emap impl
type LEMap[T Item] struct {
	mu sync.RWMutex

	bh    *heap.Heap[*bucket, int64]
	seen  map[ids.ID]struct{} // Stores a set of unique tx ids
	times map[int64]*bucket   // Uses timestamp as keys to map to buckets of ids.
}

// NewLEMap returns a pointer to a instance of an empty LEMap struct.
func NewLEMap[T Item]() *LEMap[T] {
	return &LEMap[T]{
		seen:  map[ids.ID]struct{}{},
		times: make(map[int64]*bucket),
		bh:    heap.New[*bucket, int64](120, true),
	}
}

// Add adds a list of txs to the LEMap.
func (e *LEMap[T]) Add(items []T) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, item := range items {
		e.add(item)
	}
}

// Add adds an id with a timestampt [t] to the LEMap. If the timestamp
// is genesis(0) or the id has been seen already, add returns. The id is
// added to a bucket with timestamp [t]. If no bucket exists, add creates a
// new bucket and pushes it to the binaryHeap.
func (e *LEMap[T]) add(item T) {
	id := item.ID()
	t := item.Expiry()

	// Assume genesis txs can't be placed in seen tracker
	if t == 0 {
		return
	}

	// Check if already exists
	_, ok := e.seen[id]
	if ok {
		return
	}
	e.seen[id] = struct{}{}

	// Check if bucket with time already exists
	if b, ok := e.times[t]; ok {
		b.items = append(b.items, id)
		return
	}

	// Create new bucket
	b := &bucket{
		t:     t,
		items: []ids.ID{id},
	}
	e.times[t] = b
	e.bh.Push(&heap.Entry[*bucket, int64]{
		ID:    id,
		Val:   t,
		Item:  b,
		Index: e.bh.Len(),
	})
}

// SetMin removes all buckets with a lower
// timestamp than [t] from e's bucketHeap.
func (e *LEMap[T]) SetMin(t int64) []ids.ID {
	e.mu.Lock()
	defer e.mu.Unlock()

	evicted := []ids.ID{}
	for {
		b := e.bh.First()
		if b == nil || b.Val >= t {
			break
		}
		e.bh.Pop()
		for _, id := range b.Item.items {
			delete(e.seen, id)
			evicted = append(evicted, id)
		}
		// Delete from times map
		delete(e.times, b.Val)
	}
	return evicted
}

// Any returns true if any items have been seen by LEMap.
func (e *LEMap[T]) Any(items []T) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, item := range items {
		_, ok := e.seen[item.ID()]
		if ok {
			return true
		}
	}
	return false
}

func (e *LEMap[T]) Contains(items []T, marker set.Bits, stop bool) set.Bits {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for i, item := range items {
		if marker.Contains(i) {
			continue
		}
		_, ok := e.seen[item.ID()]
		if ok {
			marker.Add(i)
			if stop {
				return marker
			}
		}
	}
	return marker
}

func (e *LEMap[T]) Has(item T) bool {
	return e.HasID(item.ID())
}

func (e *LEMap[T]) HasID(id ids.ID) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	_, ok := e.seen[id]
	return ok
}

func (e *LEMap[T]) Size() int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return len(e.seen)
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package emap

import (
	"sync"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/heap"
)

type bucket struct {
	t     int64    // Timestamp
	items []ids.ID // Array of AvalancheGo ids
}

// Item defines an interface accepted by EMap
type Item interface {
	ID() ids.ID    // method for returning an id of the item
	Expiry() int64 // method for returing this items timestamp
}

// A Emap implements en eviction map that stores the status
// of txs and their linked timestamps. The type [T] must implement the
// Item interface.
type EMap[T Item] struct {
	mu sync.RWMutex

	bh    *heap.Heap[*bucket, int64]
	seen  set.Set[ids.ID]   // Stores a set of unique tx ids
	times map[int64]*bucket // Uses timestamp as keys to map to buckets of ids.
}

// NewEMap returns a pointer to a instance of an empty EMap struct.
func NewEMap[T Item]() *EMap[T] {
	return &EMap[T]{
		seen:  set.Set[ids.ID]{},
		times: make(map[int64]*bucket),
		bh:    heap.New[*bucket, int64](120, true),
	}
}

// Add adds a list of txs to the EMap.
func (e *EMap[T]) Add(items []T) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, item := range items {
		e.add(item.ID(), item.Expiry())
	}
}

// Add adds an id with a timestampt [t] to the EMap. If the timestamp
// is genesis(0) or the id has been seen already, add returns. The id is
// added to a bucket with timestamp [t]. If no bucket exists, add creates a
// new bucket and pushes it to the binaryHeap.
func (e *EMap[T]) add(id ids.ID, t int64) {
	// Assume genesis txs can't be placed in seen tracker
	if t == 0 {
		return
	}

	// Check if already exists
	if e.seen.Contains(id) {
		return
	}
	e.seen.Add(id)

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
func (e *EMap[T]) SetMin(t int64) []ids.ID {
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
			e.seen.Remove(id)
			evicted = append(evicted, id)
		}
		// Delete from times map
		delete(e.times, b.Val)
	}
	return evicted
}

// Any returns true if any items have been seen by EMap.
func (e *EMap[T]) Any(items []T) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, item := range items {
		if e.seen.Contains(item.ID()) {
			return true
		}
	}
	return false
}

func (e *EMap[T]) Contains(items []T, marker set.Bits, stop bool) set.Bits {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for i, item := range items {
		if marker.Contains(i) {
			continue
		}
		if e.seen.Contains(item.ID()) {
			marker.Add(i)
			if stop {
				return marker
			}
		}
	}
	return marker
}

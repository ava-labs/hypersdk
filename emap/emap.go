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

	modulus int64

	bh    *heap.Heap[*bucket, int64]
	seen  set.Set[ids.ID]   // Stores a set of unique tx ids
	times map[int64]*bucket // Uses timestamp as keys to map to buckets of ids.
}

// NewEMap returns a pointer to a instance of an empty EMap struct. The provided [modulus]
// is used to limit the number of buckets that may be used to cover a time range. For example,
// a user may wish to modulo [Expiry] values denominated in ms by 1000 to convert to second-level
// granularity.
//
// Assumes [divisor] is > 0.
func NewEMap[T Item](modulus int64) *EMap[T] {
	return &EMap[T]{
		modulus: modulus,
		seen:    set.Set[ids.ID]{},
		times:   make(map[int64]*bucket),
		bh:      heap.New[*bucket, int64](120, true),
	}
}

// TODO: ensure this is inlined
// TODO: add a test for non-zero modulus
func (e *EMap[T]) transformExpiry(t int64) int64 {
	return t - t%e.modulus
}

// Add adds a list of txs to the EMap.
func (e *EMap[T]) Add(items []T) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, item := range items {
		e.add(item.ID(), e.transformExpiry(item.Expiry()))
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
func (e *EMap[T]) SetMin(tr int64) []ids.ID {
	t := e.transformExpiry(tr)

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

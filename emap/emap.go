// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package emap

import (
	"container/heap"
	"fmt"
	"sync"

	"github.com/ava-labs/avalanchego/ids"
)

type bucket struct {
	t     int64    // Timestamp
	items []ids.ID // Array of AvalancheGo ids
}

// A bucketHeap implements the heap interface to construct a priority queue.
// The heap structure is maintained by the less function which
// sorts the heap in ascending order by a buckets timestamp [t].
type bucketHeap struct {
	buckets []*bucket
}

// Len returns the length of the buckets array in bh.
func (bh *bucketHeap) Len() int {
	return len(bh.buckets)
}

// Less returns whether i's timestamp is less than j's.
func (bh *bucketHeap) Less(i, j int) bool {
	return bh.buckets[i].t < bh.buckets[j].t
}

// Swap swaps the i and jth bucket in bh.
func (bh *bucketHeap) Swap(i, j int) {
	bh.buckets[i], bh.buckets[j] = bh.buckets[j], bh.buckets[i]
}

// Pushes a *bucket to the end of bh.buckets.
// Panics if the parameter passed in is not a *bucket.
func (bh *bucketHeap) Push(x interface{}) {
	entry, ok := x.(*bucket)
	if !ok {
		panic(fmt.Errorf("unexpected %T, expected *bucket", x))
	}
	bh.buckets = append(bh.buckets, entry)
}

// Pop returns the highest priority bucket in bh and deletes it
// from the buckets array. It is called by heap.Pop() which sets the highest
// priority bucket to the last slot before calling Pop.
func (bh *bucketHeap) Pop() interface{} {
	n := len(bh.buckets)
	item := bh.buckets[n-1]
	bh.buckets[n-1] = nil // avoid memory leak
	bh.buckets = bh.buckets[0 : n-1]
	return item
}

// Peek returns the first bucket in bh. If bh has no
// buckets, Peek returns nil.
func (bh *bucketHeap) Peek() *bucket {
	if len(bh.buckets) == 0 {
		return nil
	}
	return bh.buckets[0]
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

	bh    *bucketHeap
	seen  map[ids.ID]struct{} // Stores a set of unique tx ids
	times map[int64]*bucket   // Uses timestamp as keys to map to buckets of ids.
}

// NewEMap returns a pointer to a instance of an empty EMap struct.
func NewEMap[T Item]() *EMap[T] {
	return &EMap[T]{
		seen:  make(map[ids.ID]struct{}),
		times: make(map[int64]*bucket),
		bh: &bucketHeap{
			buckets: []*bucket{},
		},
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
	if _, ok := e.seen[id]; ok {
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
	heap.Push(e.bh, b)
}

// SetMin removes all buckets with a lower
// timestamp than [t] from e's bucketHeap.
func (e *EMap[T]) SetMin(t int64) []ids.ID {
	e.mu.Lock()
	defer e.mu.Unlock()

	evicted := []ids.ID{}
	for {
		b := e.bh.Peek()
		if b == nil || b.t >= t {
			break
		}
		heap.Pop(e.bh)
		for _, id := range b.items {
			delete(e.seen, id)
			evicted = append(evicted, id)
		}
		// Delete from times map
		delete(e.times, b.t)
	}
	return evicted
}

// Any returns true if any items have been seen by EMap.
func (e *EMap[T]) Any(items []T) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, item := range items {
		if _, ok := e.seen[item.ID()]; ok {
			return true
		}
	}
	return false
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package eheap

import (
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/heap"
)

// Item is the interface that any item put in the heap must adheare to.
type Item interface {
	ID() ids.ID
	Expiry() int64
}

// ExpiryHeap keeps a min heap of [Items] sorted by [Expiry].
//
// This data structure is similar to [emap] with the exception that
// items can be removed (ExpiryHeap must store each item in the heap
// instead of grouping by expiry to support this feature, which makes it
// less efficient).
type ExpiryHeap[T Item] struct {
	minHeap *heap.Heap[T, int64]
}

// New returns an instance of ExpiryHeap with minHeap and maxHeap
// containing [items].
func New[T Item](items int) *ExpiryHeap[T] {
	return &ExpiryHeap[T]{
		minHeap: heap.New[T, int64](items, true),
	}
}

// Add pushes [item] to eh.
func (eh *ExpiryHeap[T]) Add(item T) {
	itemID := item.ID()
	poolLen := eh.minHeap.Len()
	eh.minHeap.Push(&heap.Entry[T, int64]{
		ID:    itemID,
		Val:   item.Expiry(),
		Item:  item,
		Index: poolLen,
	})
}

// Remove removes [id] from eh. If the id does not exist, Remove returns.
func (eh *ExpiryHeap[T]) Remove(id ids.ID) (T, bool) {
	minEntry, ok := eh.minHeap.Get(id) // O(1)
	if !ok {
		// This should never happen, as that would mean the heaps are out of
		// sync.
		return *new(T), false
	}
	eh.minHeap.Remove(minEntry.Index) // O(log N)
	return minEntry.Item, true
}

// SetMin removes all elements in eh with a value less than [val]. Returns
// the list of removed elements.
func (eh *ExpiryHeap[T]) SetMin(val int64) []T {
	removed := []T{}
	for {
		min, ok := eh.PeekMin()
		if !ok {
			break
		}
		if min.Expiry() < val {
			eh.PopMin() // Assumes that there is not concurrent access to [ExpiryHeap]
			removed = append(removed, min)
			continue
		}
		break
	}
	return removed
}

// PeekMin returns the minimum value in eh.
func (eh *ExpiryHeap[T]) PeekMin() (T, bool) {
	first := eh.minHeap.First()
	if first == nil {
		return *new(T), false
	}
	return first.Item, true
}

// PopMin removes the minimum value in eh.
func (eh *ExpiryHeap[T]) PopMin() (T, bool) {
	first := eh.minHeap.First()
	if first == nil {
		return *new(T), false
	}
	item := first.Item
	eh.Remove(item.ID())
	return item, true
}

// Has returns if [item] is in eh.
func (eh *ExpiryHeap[T]) Has(item ids.ID) bool {
	return eh.minHeap.Has(item)
}

// Len returns the number of elements in eh.
func (eh *ExpiryHeap[T]) Len() int {
	return eh.minHeap.Len()
}

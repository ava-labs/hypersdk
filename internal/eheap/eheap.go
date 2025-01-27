// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package eheap

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/heap"
)

// Item is the interface that any item put in the heap must adheare to.
type Item interface {
	GetID() ids.ID
	GetExpiry() int64
}

// ExpiryHeap keeps a min heap of [Items] sorted by [Expiry].
//
// This data structure is similar to [emap] with the exception that
// items can be removed (ExpiryHeap must store each item in the heap
// instead of grouping by expiry to support this feature, which makes it
// less efficient).
type ExpiryHeap[T Item] struct {
	minHeap heap.Map[ids.ID, T]
}

// New returns an instance of ExpiryHeap with minHeap and maxHeap
// containing [items].
func New[T Item]() *ExpiryHeap[T] {
	return &ExpiryHeap[T]{
		minHeap: heap.NewMap[ids.ID, T](func(a, b T) bool {
			return a.GetExpiry() < b.GetExpiry()
		}),
	}
}

// Add pushes [item] to eh.
func (eh *ExpiryHeap[T]) Add(item T) {
	eh.minHeap.Push(item.GetID(), item)
}

// Remove removes [id] from eh. If the id does not exist, Remove returns.
func (eh *ExpiryHeap[T]) Remove(id ids.ID) (T, bool) {
	return eh.minHeap.Remove(id)
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
		if min.GetExpiry() < val {
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
	_, v, ok := eh.minHeap.Peek()
	return v, ok
}

// PopMin removes the minimum value in eh.
func (eh *ExpiryHeap[T]) PopMin() (T, bool) {
	_, v, ok := eh.minHeap.Pop()
	return v, ok
}

// Has returns if [item] is in eh.
func (eh *ExpiryHeap[T]) Has(item ids.ID) bool {
	return eh.minHeap.Contains(item)
}

// Len returns the number of elements in eh.
func (eh *ExpiryHeap[T]) Len() int {
	return eh.minHeap.Len()
}

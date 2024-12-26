// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pheap

import (
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/internal/heap"
)

// Item is the interface that any item put in the heap must adheare to.
type Item interface {
	GetID() ids.ID
	Priority() uint64
}

// PriorityHeap keeps a max heap of [Items] sorted by [PriorityFees].
type PriorityHeap[T Item] struct {
	maxHeap *heap.Heap[T, uint64]
}

// New returns an instance of PriorityHeap with maxHeap containing [items].
func New[T Item](items int) *PriorityHeap[T] {
	return &PriorityHeap[T]{
		maxHeap: heap.New[T, uint64](items, false),
	}
}

// Add pushes [item] to ph.
func (ph *PriorityHeap[T]) Add(item T) {
	poolLen := ph.maxHeap.Len()
	ph.maxHeap.Push(&heap.Entry[T, uint64]{
		ID:    item.GetID(),
		Val:   item.Priority(),
		Item:  item,
		Index: poolLen,
	})
}

// Remove removes [id] from ph. If the id does not exist, Remove returns.
func (ph *PriorityHeap[T]) Remove(id ids.ID) (T, bool) {
	entry, ok := ph.maxHeap.Get(id) // O(1)
	if !ok {
		// This should never happen, as that would mean the heaps are out of
		// sync.
		return *new(T), false
	}
	ph.maxHeap.Remove(entry.Index) // O(log N)
	return entry.Item, true
}

// PopMax removes the maximum value in ph.
func (ph *PriorityHeap[T]) Pop() (T, bool) {
	entry := ph.maxHeap.Pop()
	if entry == nil {
		return *new(T), false
	}
	return entry.Item, true
}

// Has returns if [item] is in ph.
func (ph *PriorityHeap[T]) Has(item ids.ID) bool {
	return ph.maxHeap.Has(item)
}

// Len returns the number of elements in ph.
func (ph *PriorityHeap[T]) Len() int {
	return ph.maxHeap.Len()
}

// First returns the maximum value in ph.
func (ph *PriorityHeap[T]) First() (T, bool) {
	first := ph.maxHeap.First()
	if first == nil {
		return *new(T), false
	}
	return first.Item, true
}

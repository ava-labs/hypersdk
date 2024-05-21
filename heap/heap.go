// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package heap

import (
	"cmp"
	"container/heap"

	"github.com/ava-labs/avalanchego/ids"
)

// Heap[I,V] is used to track objects of [I] by [Val].
//
// This data structure does not perform any synchronization and is not
// safe to use concurrently without external locking.
type Heap[I any, V cmp.Ordered] struct {
	ih *innerHeap[I, V]
}

// New returns an instance of Heap[I,V]
func New[I any, V cmp.Ordered](items int, isMinHeap bool) *Heap[I, V] {
	return &Heap[I, V]{newInnerHeap[I, V](items, isMinHeap)}
}

// Len returns the number of items in ih.
func (h *Heap[I, V]) Len() int { return h.ih.Len() }

// Get returns the entry in th associated with [id], and a bool if [id] was
// found in th.
func (h *Heap[I, V]) Get(id ids.ID) (*Entry[I, V], bool) {
	return h.ih.Get(id)
}

// Has returns whether [id] is found in th.
func (h *Heap[I, V]) Has(id ids.ID) bool {
	return h.ih.Has(id)
}

// Items returns all items in the heap in sorted order. You should not modify
// the response.
func (h *Heap[I, V]) Items() []*Entry[I, V] {
	return h.ih.items
}

// Push can be called by external users instead of using `containers.heap`,
// which makes using this heap less error-prone.
func (h *Heap[I, V]) Push(e *Entry[I, V]) {
	heap.Push(h.ih, e)
}

// Pop can be called by external users to remove an object from the heap at
// a specific index instead of using `containers.heap`,
// which makes using this heap less error-prone.
func (h *Heap[I, V]) Pop() *Entry[I, V] {
	if len(h.ih.items) == 0 {
		return nil
	}
	return heap.Pop(h.ih).(*Entry[I, V])
}

// Remove can be called by external users to remove an object from the heap at
// a specific index instead of using `containers.heap`,
// which makes using this heap less error-prone.
func (h *Heap[I, V]) Remove(index int) *Entry[I, V] {
	if index >= len(h.ih.items) {
		return nil
	}
	return heap.Remove(h.ih, index).(*Entry[I, V])
}

// First returns the first item in the heap. This is the smallest item in
// a minHeap and the largest item in a maxHeap.
//
// If no items are in the heap, it will return nil.
func (h *Heap[I, V]) First() *Entry[I, V] {
	if len(h.ih.items) == 0 {
		return nil
	}
	return h.ih.items[0]
}

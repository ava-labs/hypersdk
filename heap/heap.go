// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package heap

import (
	"cmp"
	"container/heap"
)

// Heap[I,V] is used to track objects of [I] by [Val].
//
// This data structure does not perform any synchronization and is not
// safe to use concurrently without external locking.
type Heap[T comparable, I any, V cmp.Ordered] struct {
	ih *innerHeap[T, I, V]
}

// New returns an instance of Heap[I,V]
func New[T comparable, I any, V cmp.Ordered](items int, isMinHeap bool) *Heap[T, I, V] {
	return &Heap[T, I, V]{newInnerHeap[T, I, V](items, isMinHeap)}
}

// Len returns the number of items in ih.
func (h *Heap[T, I, V]) Len() int { return h.ih.Len() }

// Get returns the entry in th associated with [id], and a bool if [id] was
// found in th.
func (h *Heap[T, I, V]) Get(id T) (*Entry[T, I, V], bool) {
	return h.ih.Get(id)
}

// Has returns whether [id] is found in th.
func (h *Heap[T, I, V]) Has(id T) bool {
	return h.ih.Has(id)
}

// Items returns all items in the heap in sorted order. You should not modify
// the response.
func (h *Heap[T, I, V]) Items() []*Entry[T, I, V] {
	return h.ih.items
}

// Push can be called by external users instead of using `containers.heap`,
// which makes using this heap less error-prone.
func (h *Heap[T, I, V]) Push(e *Entry[T, I, V]) {
	heap.Push(h.ih, e)
}

// Pop can be called by external users to remove an object from the heap at
// a specific index instead of using `containers.heap`,
// which makes using this heap less error-prone.
func (h *Heap[T, I, V]) Pop() *Entry[T, I, V] {
	if len(h.ih.items) == 0 {
		return nil
	}
	return heap.Pop(h.ih).(*Entry[T, I, V])
}

// Remove can be called by external users to remove an object from the heap at
// a specific index instead of using `containers.heap`,
// which makes using this heap less error-prone.
func (h *Heap[T, I, V]) Remove(index int) *Entry[T, I, V] {
	if index >= len(h.ih.items) {
		return nil
	}
	return heap.Remove(h.ih, index).(*Entry[T, I, V])
}

// First returns the first item in the heap. This is the smallest item in
// a minHeap and the largest item in a maxHeap.
//
// If no items are in the heap, it will return nil.
func (h *Heap[T, I, V]) First() *Entry[T, I, V] {
	if len(h.ih.items) == 0 {
		return nil
	}
	return h.ih.items[0]
}

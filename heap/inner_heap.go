// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package heap

import (
	"cmp"
	"container/heap"
	"fmt"
)

var _ heap.Interface = (*innerHeap[any, any, uint64])(nil)

type Entry[T comparable, I any, V cmp.Ordered] struct {
	ID   T // id of entry
	Item I // associated item
	Val  V // Value to be prioritized

	Index int // Index of the entry in heap
}

type innerHeap[T comparable, I any, V cmp.Ordered] struct {
	isMinHeap bool                  // true for Min-Heap, false for Max-Heap
	items     []*Entry[T, I, V]     // items in this heap
	lookup    map[T]*Entry[T, I, V] // ids in the heap mapping to an entry
}

func newInnerHeap[T comparable, I any, V cmp.Ordered](items int, isMinHeap bool) *innerHeap[T, I, V] {
	return &innerHeap[T, I, V]{
		isMinHeap: isMinHeap,

		items:  make([]*Entry[T, I, V], 0, items),
		lookup: make(map[T]*Entry[T, I, V], items),
	}
}

// Len returns the number of items in ih.
func (ih *innerHeap[T, I, V]) Len() int { return len(ih.items) }

// Less compares the priority of [i] and [j] based on th.isMinHeap.
//
// This should never be called by an external caller and is required to
// confirm to `heap.Interface`.
func (ih *innerHeap[T, I, V]) Less(i, j int) bool {
	if ih.isMinHeap {
		return ih.items[i].Val < ih.items[j].Val
	}
	return ih.items[i].Val > ih.items[j].Val
}

// Swap swaps the [i]th and [j]th element in th.
//
// This should never be called by an external caller and is required to
// confirm to `heap.Interface`.
func (ih *innerHeap[T, I, V]) Swap(i, j int) {
	ih.items[i], ih.items[j] = ih.items[j], ih.items[i]
	ih.items[i].Index = i
	ih.items[j].Index = j
}

// Push adds an *Entry interface to th. If [x.id] is already in
// th, returns.
//
// This should never be called by an external caller and is required to
// confirm to `heap.Interface`.
func (ih *innerHeap[T, I, V]) Push(x any) {
	entry, ok := x.(*Entry[T, I, V])
	if !ok {
		panic(fmt.Errorf("unexpected %T, expected *Entry", x))
	}
	if ih.Has(entry.ID) {
		return
	}
	ih.items = append(ih.items, entry)
	ih.lookup[entry.ID] = entry
}

// Pop removes the highest priority item from th and also deletes it from
// th's lookup map.
//
// This should never be called by an external caller and is required to
// confirm to `heap.Interface`.
func (ih *innerHeap[T, I, V]) Pop() any {
	n := len(ih.items)
	item := ih.items[n-1]
	ih.items[n-1] = nil // avoid memory leak
	ih.items = ih.items[0 : n-1]
	delete(ih.lookup, item.ID)
	return item
}

// Get returns the entry in th associated with [id], and a bool if [id] was
// found in th.
func (ih *innerHeap[T, I, V]) Get(id T) (*Entry[T, I, V], bool) {
	entry, ok := ih.lookup[id]
	return entry, ok
}

// Has returns whether [id] is found in th.
func (ih *innerHeap[T, I, V]) Has(id T) bool {
	_, has := ih.Get(id)
	return has
}

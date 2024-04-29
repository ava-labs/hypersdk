// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package heap

import (
	"cmp"
	"container/heap"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
)

var _ heap.Interface = (*innerHeap[any, uint64])(nil)

type Entry[I any, V cmp.Ordered] struct {
	ID   ids.ID // id of entry
	Item I      // associated item
	Val  V      // Value to be prioritized

	Index int // Index of the entry in heap
}

type innerHeap[I any, V cmp.Ordered] struct {
	isMinHeap bool                    // true for Min-Heap, false for Max-Heap
	items     []*Entry[I, V]          // items in this heap
	lookup    map[ids.ID]*Entry[I, V] // ids in the heap mapping to an entry
}

func newInnerHeap[I any, V cmp.Ordered](items int, isMinHeap bool) *innerHeap[I, V] {
	return &innerHeap[I, V]{
		isMinHeap: isMinHeap,

		items:  make([]*Entry[I, V], 0, items),
		lookup: make(map[ids.ID]*Entry[I, V], items),
	}
}

// Len returns the number of items in ih.
func (ih *innerHeap[I, V]) Len() int { return len(ih.items) }

// Less compares the priority of [i] and [j] based on th.isMinHeap.
//
// This should never be called by an external caller and is required to
// confirm to `heap.Interface`.
func (ih *innerHeap[I, V]) Less(i, j int) bool {
	if ih.isMinHeap {
		return ih.items[i].Val < ih.items[j].Val
	}
	return ih.items[i].Val > ih.items[j].Val
}

// Swap swaps the [i]th and [j]th element in th.
//
// This should never be called by an external caller and is required to
// confirm to `heap.Interface`.
func (ih *innerHeap[I, V]) Swap(i, j int) {
	ih.items[i], ih.items[j] = ih.items[j], ih.items[i]
	ih.items[i].Index = i
	ih.items[j].Index = j
}

// Push adds an *Entry interface to th. If [x.id] is already in
// th, returns.
//
// This should never be called by an external caller and is required to
// confirm to `heap.Interface`.
func (ih *innerHeap[I, V]) Push(x any) {
	entry, ok := x.(*Entry[I, V])
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
func (ih *innerHeap[I, V]) Pop() any {
	n := len(ih.items)
	item := ih.items[n-1]
	ih.items[n-1] = nil // avoid memory leak
	ih.items = ih.items[0 : n-1]
	delete(ih.lookup, item.ID)
	return item
}

// Get returns the entry in th associated with [id], and a bool if [id] was
// found in th.
func (ih *innerHeap[I, V]) Get(id ids.ID) (*Entry[I, V], bool) {
	entry, ok := ih.lookup[id]
	return entry, ok
}

// Has returns whether [id] is found in th.
func (ih *innerHeap[I, V]) Has(id ids.ID) bool {
	_, has := ih.Get(id)
	return has
}

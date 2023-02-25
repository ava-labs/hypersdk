// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package heap

import (
	"container/heap"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"golang.org/x/exp/constraints"
)

var _ heap.Interface = (*Heap[interface{}, uint64])(nil)

type Entry[I any, V constraints.Ordered] struct {
	ID   ids.ID // id of entry
	Item I      // associated item
	Val  V      // Value to be prioritized

	Index int // Index of the entry in heap
}

// Heap[I,V] is used to track objectes of [I] by [Val].
type Heap[I any, V constraints.Ordered] struct {
	isMinHeap bool                    // true for Min-Heap, false for Max-Heap
	items     []*Entry[I, V]          // items in this heap
	lookup    map[ids.ID]*Entry[I, V] // ids in the heap mapping to an entry
}

// New returns an instance of Heap[I,V]
func New[I any, V constraints.Ordered](items int, isMinHeap bool) *Heap[I, V] {
	return &Heap[I, V]{
		isMinHeap: isMinHeap,

		items:  make([]*Entry[I, V], 0, items),
		lookup: make(map[ids.ID]*Entry[I, V], items),
	}
}

// Len returns the number of items in th.
func (th *Heap[I, V]) Len() int { return len(th.items) }

// Less compares the priority of [i] and [j] based on th.isMinHeap.
func (th *Heap[I, V]) Less(i, j int) bool {
	if th.isMinHeap {
		return th.items[i].Val < th.items[j].Val
	}
	return th.items[i].Val > th.items[j].Val
}

// Swap swaps the [i]th and [j]th element in th.
func (th *Heap[I, V]) Swap(i, j int) {
	th.items[i], th.items[j] = th.items[j], th.items[i]
	th.items[i].Index = i
	th.items[j].Index = j
}

// Push adds an *Entry interface to th. If [x.id] is already in
// th, returns.
func (th *Heap[I, V]) Push(x interface{}) {
	entry, ok := x.(*Entry[I, V])
	if !ok {
		panic(fmt.Errorf("unexpected %T, expected *Uint64Entry", x))
	}
	if th.HasID(entry.ID) {
		return
	}
	th.items = append(th.items, entry)
	th.lookup[entry.ID] = entry
}

// Pop removes the highest priority item from th and also deletes it from
// th's lookup map.
func (th *Heap[I, V]) Pop() interface{} {
	n := len(th.items)
	item := th.items[n-1]
	th.items[n-1] = nil // avoid memory leak
	th.items = th.items[0 : n-1]
	delete(th.lookup, item.ID)
	return item
}

// GetID returns the entry in th associated with [id], and a bool if [id] was
// found in th.
func (th *Heap[I, V]) GetID(id ids.ID) (*Entry[I, V], bool) {
	entry, ok := th.lookup[id]
	return entry, ok
}

// HasID returns whether [id] is found in th.
func (th *Heap[I, V]) HasID(id ids.ID) bool {
	_, has := th.GetID(id)
	return has
}

// Items returns all items in the heap in sorted order. You should not modify
// the response.
func (th *Heap[I, V]) Items() []*Entry[I, V] {
	return th.items
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
)

type Item interface {
	ID() ids.ID
	Payer() string
	Expiry() int64
	UnitPrice() uint64
}

type uint64Entry[T Item] struct {
	id   ids.ID // id of entry
	item T      // associated item
	val  uint64 // value to be prioritized

	index int // index of the entry in heap
}

// uint64Heap[T] is used to track pending transactions by [val]
type uint64Heap[T Item] struct {
	isMinHeap bool                       // true for Min-Heap, false for Max-Heap
	items     []*uint64Entry[T]          // items in this heap
	lookup    map[ids.ID]*uint64Entry[T] // ids in the heap mapping to an entry
}

// newUint64Heap returns an instance of uint64Heap[T]
func newUint64Heap[T Item](items int, isMinHeap bool) *uint64Heap[T] {
	return &uint64Heap[T]{
		isMinHeap: isMinHeap,

		items:  make([]*uint64Entry[T], 0, items),
		lookup: make(map[ids.ID]*uint64Entry[T], items),
	}
}

// Len returns the number of items in th.
func (th uint64Heap[T]) Len() int { return len(th.items) }

// Less compares the priority of [i] and [j] based on th.isMinHeap.
func (th uint64Heap[T]) Less(i, j int) bool {
	if th.isMinHeap {
		return th.items[i].val < th.items[j].val
	}
	return th.items[i].val > th.items[j].val
}

// Swap swaps the [i]th and [j]th element in th.
func (th uint64Heap[T]) Swap(i, j int) {
	th.items[i], th.items[j] = th.items[j], th.items[i]
	th.items[i].index = i
	th.items[j].index = j
}

// Push addes an *uint64Entry interface to th. If [x.id] is already in
// th, returns.
func (th *uint64Heap[T]) Push(x interface{}) {
	entry, ok := x.(*uint64Entry[T])
	if !ok {
		panic(fmt.Errorf("unexpected %T, expected *uint64Entry", x))
	}
	if th.HasID(entry.id) {
		return
	}
	th.items = append(th.items, entry)
	th.lookup[entry.id] = entry
}

// Pop removes the highest priority item from th and also deletes it from
// th's lookup map.
func (th *uint64Heap[T]) Pop() interface{} {
	n := len(th.items)
	item := th.items[n-1]
	th.items[n-1] = nil // avoid memory leak
	th.items = th.items[0 : n-1]
	delete(th.lookup, item.id)
	return item
}

// GetID returns the entry in th associated with [id], and a bool if [id] was
// found in th.
func (th *uint64Heap[T]) GetID(id ids.ID) (*uint64Entry[T], bool) {
	entry, ok := th.lookup[id]
	return entry, ok
}

// HasID returns whether [id] is found in th.
func (th *uint64Heap[T]) HasID(id ids.ID) bool {
	_, has := th.GetID(id)
	return has
}

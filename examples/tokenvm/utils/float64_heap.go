// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package utils

import (
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
)

// TODO: make Val generic and move to shared utils package in HyperSDK
type Float64Entry[T any] struct {
	ID   ids.ID  // id of entry
	Item T       // associated item
	Val  float64 // Value to be prioritized

	Index int // Index of the entry in heap
}

// Float64Heap[T] is used to track pending transactions by [Val]
type Float64Heap[T any] struct {
	isMinHeap bool                        // true for Min-Heap, false for Max-Heap
	items     []*Float64Entry[T]          // items in this heap
	lookup    map[ids.ID]*Float64Entry[T] // ids in the heap mapping to an entry
}

// newFloat64Heap returns an instance of Float64Heap[T]
func NewFloat64Heap[T any](items int, isMinHeap bool) *Float64Heap[T] {
	return &Float64Heap[T]{
		isMinHeap: isMinHeap,

		items:  make([]*Float64Entry[T], 0, items),
		lookup: make(map[ids.ID]*Float64Entry[T], items),
	}
}

// Len returns the number of items in th.
func (th Float64Heap[T]) Len() int {
	return len(th.items)
}

// Less compares the priority of [i] and [j] based on th.isMinHeap.
func (th Float64Heap[T]) Less(i, j int) bool {
	if th.isMinHeap {
		return th.items[i].Val < th.items[j].Val
	}
	return th.items[i].Val > th.items[j].Val
}

// Swap swaps the [i]th and [j]th element in th.
func (th Float64Heap[T]) Swap(i, j int) {
	th.items[i], th.items[j] = th.items[j], th.items[i]
	th.items[i].Index = i
	th.items[j].Index = j
}

// Push addes an *Float64Entry interface to th. If [x.ID] is already in
// th, returns.
func (th *Float64Heap[T]) Push(x interface{}) {
	entry, ok := x.(*Float64Entry[T])
	if !ok {
		panic(fmt.Errorf("unexpected %T, expected *Float64Entry", x))
	}
	if th.HasID(entry.ID) {
		return
	}
	th.items = append(th.items, entry)
	th.lookup[entry.ID] = entry
}

// Pop removes the highest priority item from th and also deletes it from
// th's lookup map.
func (th *Float64Heap[T]) Pop() interface{} {
	n := len(th.items)
	item := th.items[n-1]
	th.items[n-1] = nil // avoid memory leak
	th.items = th.items[0 : n-1]
	delete(th.lookup, item.ID)
	return item
}

// GetID returns the entry in th associated with [id], and a bool if [id] was
// found in th.
func (th *Float64Heap[T]) GetID(id ids.ID) (*Float64Entry[T], bool) {
	entry, ok := th.lookup[id]
	return entry, ok
}

// HasID returns whether [id] is found in th.
func (th *Float64Heap[T]) HasID(id ids.ID) bool {
	_, has := th.GetID(id)
	return has
}

// Items returns all items in the heap in sorted order. You should not modify
// the response.
func (th *Float64Heap[T]) Items() []*Float64Entry[T] {
	return th.items
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"container/heap"

	"github.com/ava-labs/avalanchego/ids"
)

// SortedMempool contains a max-heap and min-heap. The order within each
// heap is determined by using GetValue.
type SortedMempool[T Item] struct {
	// GetValue informs heaps how to get the an entry's value for ordering.
	GetValue func(item T) uint64

	minHeap *uint64Heap[T] // only includes lowest nonce
	maxHeap *uint64Heap[T] // only includes lowest nonce
}

// NewSortedMempool returns an instance of SortedMempool with minHeap and maxHeap
// containing [items] and prioritized with [f]
func NewSortedMempool[T Item](items int, f func(item T) uint64) *SortedMempool[T] {
	return &SortedMempool[T]{
		GetValue: f,
		minHeap:  newUint64Heap[T](items, true),
		maxHeap:  newUint64Heap[T](items, false),
	}
}

// Add pushes [item] to sm.
func (sm *SortedMempool[T]) Add(item T) {
	itemID := item.ID()
	poolLen := sm.maxHeap.Len()
	val := sm.GetValue(item)
	heap.Push(sm.maxHeap, &uint64Entry[T]{
		id:    itemID,
		val:   val,
		item:  item,
		index: poolLen,
	})
	heap.Push(sm.minHeap, &uint64Entry[T]{
		id:    itemID,
		val:   val,
		item:  item,
		index: poolLen,
	})
}

// Remove removes [id] from sm. If the id does not exist, Remove returns.
func (sm *SortedMempool[T]) Remove(id ids.ID) {
	maxEntry, ok := sm.maxHeap.GetID(id) // O(1)
	if !ok {
		return
	}
	heap.Remove(sm.maxHeap, maxEntry.index) // O(log N)
	minEntry, ok := sm.minHeap.GetID(id)
	if !ok {
		// This should never happen, as that would mean the heaps are out of
		// sync.
		return
	}
	heap.Remove(sm.minHeap, minEntry.index) // O(log N)
}

// SetMinVal removes all elements in sm with a value less than [val]. Returns
// the list of removed elements.
// TODO: add lock to prevent concurrent access
func (sm *SortedMempool[T]) SetMinVal(val uint64) []T {
	removed := []T{}
	for {
		min, ok := sm.PeekMin()
		if !ok {
			break
		}
		if sm.GetValue(min) < val {
			sm.PopMin() // Assumes that there is not concurrent access to [SortedMempool]
			removed = append(removed, min)
			continue
		}
		break
	}
	return removed
}

// PeekMin returns the minimum value in sm.
func (sm *SortedMempool[T]) PeekMin() (T, bool) {
	if sm.minHeap.Len() == 0 {
		return *new(T), false //nolint:gocritic
	}
	return sm.minHeap.items[0].item, true
}

// PopMin removes the minimum value in sm.
func (sm *SortedMempool[T]) PopMin() (T, bool) {
	if sm.minHeap.Len() == 0 {
		return *new(T), false //nolint:gocritic
	}
	item := sm.minHeap.items[0].item
	sm.Remove(item.ID())
	return item, true
}

// PopMin returns the maximum value in sm.
func (sm *SortedMempool[T]) PeekMax() (T, bool) {
	if sm.Len() == 0 {
		return *new(T), false //nolint:gocritic
	}
	return sm.maxHeap.items[0].item, true
}

// PopMin removes the maximum value in sm.
func (sm *SortedMempool[T]) PopMax() (T, bool) {
	if sm.Len() == 0 {
		return *new(T), false //nolint:gocritic
	}
	item := sm.maxHeap.items[0].item
	sm.Remove(item.ID())
	return item, true
}

// Has returns if [item] is in sm.
func (sm *SortedMempool[T]) Has(item ids.ID) bool {
	return sm.minHeap.HasID(item)
}

// Len returns the number of elements in sm.
func (sm *SortedMempool[T]) Len() int {
	return sm.minHeap.Len()
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/heap"
)

// Item is the interface that any item put in the mempool must adheare to.
type MempoolItem interface {
	ID() ids.ID
	Expiry() int64
}

// SortedMempool contains a max-heap and min-heap. The order within each
// heap is determined by using GetValue.
//
// This data structure does not perform any synchronization and is not
// safe to use concurrently without external locking.
type SortedMempool[T MempoolItem] struct {
	// GetValue informs heaps how to get the an entry's value for ordering.
	GetValue func(item T) uint64

	minHeap *heap.Heap[T, uint64]
}

// NewSortedMempool returns an instance of SortedMempool with minHeap and maxHeap
// containing [items] and prioritized with [f]
func NewSortedMempool[T MempoolItem](items int, f func(item T) uint64) *SortedMempool[T] {
	return &SortedMempool[T]{
		GetValue: f,
		minHeap:  heap.New[T, uint64](items, true),
	}
}

// Add pushes [item] to sm.
func (sm *SortedMempool[T]) Add(item T) {
	itemID := item.ID()
	poolLen := sm.minHeap.Len()
	val := sm.GetValue(item)
	sm.minHeap.Push(&heap.Entry[T, uint64]{
		ID:    itemID,
		Val:   val,
		Item:  item,
		Index: poolLen,
	})
}

// Remove removes [id] from sm. If the id does not exist, Remove returns.
func (sm *SortedMempool[T]) Remove(id ids.ID) (T, bool) {
	minEntry, ok := sm.minHeap.Get(id) // O(1)
	if !ok {
		// This should never happen, as that would mean the heaps are out of
		// sync.
		return *new(T), false
	}
	sm.minHeap.Remove(minEntry.Index) // O(log N)
	return minEntry.Item, true
}

// SetMinVal removes all elements in sm with a value less than [val]. Returns
// the list of removed elements.
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
	first := sm.minHeap.First()
	if first == nil {
		return *new(T), false
	}
	return first.Item, true
}

// PopMin removes the minimum value in sm.
func (sm *SortedMempool[T]) PopMin() (T, bool) {
	first := sm.minHeap.First()
	if first == nil {
		return *new(T), false
	}
	item := first.Item
	sm.Remove(item.ID())
	return item, true
}

// Has returns if [item] is in sm.
func (sm *SortedMempool[T]) Has(item ids.ID) bool {
	return sm.minHeap.Has(item)
}

// Len returns the number of elements in sm.
func (sm *SortedMempool[T]) Len() int {
	return sm.minHeap.Len()
}

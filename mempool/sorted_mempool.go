// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"container/heap"

	"github.com/ava-labs/avalanchego/ids"
)

// SortedMempool contains a max-heap and min-heap. The order within each
// heap is determined by using GetValue.
type SortedMempool struct {
	// GetValue informs heaps how to get the an entry's value for ordering.
	GetValue func(tx Item) uint64

	minHeap *uint64Heap // only includes lowest nonce
	maxHeap *uint64Heap // only includes lowest nonce
}

// NewSortedMempool returns an instance of SortedMempool with minHeap and maxHeap
// containing [items] and prioritized with [f]
func NewSortedMempool(items int, f func(tx Item) uint64) *SortedMempool {
	return &SortedMempool{
		GetValue: f,
		minHeap:  newUint64Heap(items, true),
		maxHeap:  newUint64Heap(items, false),
	}
}

// Add pushes [tx] to sm.
func (sm *SortedMempool) Add(tx Item) {
	txID := tx.ID()
	poolLen := sm.maxHeap.Len()
	val := sm.GetValue(tx)
	heap.Push(sm.maxHeap, &uint64Entry{
		id:    txID,
		val:   val,
		tx:    tx,
		index: poolLen,
	})
	heap.Push(sm.minHeap, &uint64Entry{
		id:    txID,
		val:   val,
		tx:    tx,
		index: poolLen,
	})
}

// Remove removes [id] from sm. If the id does not exist, Remove returns.
func (sm *SortedMempool) Remove(id ids.ID) {
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
// TDOD: add lock to prevent concurrent access
func (sm *SortedMempool) SetMinVal(val uint64) []Item {
	removed := []Item{}
	for {
		min := sm.PeekMin()
		if min == nil {
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

// PopMin removes the minimum value in sm.
func (sm *SortedMempool) PeekMin() Item {
	if sm.minHeap.Len() == 0 {
		return nil
	}
	return sm.minHeap.items[0].tx
}

// PopMin removes the minimum value in sm.
func (sm *SortedMempool) PopMin() Item {
	if sm.minHeap.Len() == 0 {
		return nil
	}
	tx := sm.minHeap.items[0].tx
	sm.Remove(tx.ID())
	return tx
}

// PopMin returms the maximum value in sm.
func (sm *SortedMempool) PeekMax() Item {
	if sm.Len() == 0 {
		return nil
	}
	return sm.maxHeap.items[0].tx
}

// PopMin removes the maximum value in sm.
func (sm *SortedMempool) PopMax() Item {
	if sm.Len() == 0 {
		return nil
	}
	tx := sm.maxHeap.items[0].tx
	sm.Remove(tx.ID())
	return tx
}

// Has returns if [tx] is in sm.
func (sm *SortedMempool) Has(tx ids.ID) bool {
	return sm.minHeap.HasID(tx)
}

// Len returns the number of elements in sm.
func (sm *SortedMempool) Len() int {
	return sm.minHeap.Len()
}

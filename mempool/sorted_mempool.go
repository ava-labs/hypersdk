// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"container/heap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
)

type SortedMempool struct {
	f func(tx *chain.Transaction) uint64

	minHeap *uint64Heap // only includes lowest nonce
	maxHeap *uint64Heap // only includes lowest nonce
}

func NewSortedMempool(items int, f func(tx *chain.Transaction) uint64) *SortedMempool {
	return &SortedMempool{
		f:       f,
		minHeap: newUint64Heap(items, true),
		maxHeap: newUint64Heap(items, false),
	}
}

func (sm *SortedMempool) Add(tx *chain.Transaction) {
	txID := tx.ID()
	poolLen := sm.maxHeap.Len()
	val := sm.f(tx)
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

func (sm *SortedMempool) SetMinVal(val uint64) []*chain.Transaction {
	removed := []*chain.Transaction{}
	for {
		min := sm.PeekMin()
		if min == nil {
			break
		}
		if sm.f(min) < val {
			sm.PopMin() // Assumes that there is not concurrent access to [SortedMempool]
			removed = append(removed, min)
			continue
		}
		break
	}
	return removed
}

func (sm *SortedMempool) PeekMin() *chain.Transaction {
	if sm.minHeap.Len() == 0 {
		return nil
	}
	return sm.minHeap.items[0].tx
}

func (sm *SortedMempool) PopMin() *chain.Transaction {
	if sm.minHeap.Len() == 0 {
		return nil
	}
	tx := sm.minHeap.items[0].tx
	sm.Remove(tx.ID())
	return tx
}

func (sm *SortedMempool) PeekMax() *chain.Transaction {
	if sm.Len() == 0 {
		return nil
	}
	return sm.maxHeap.items[0].tx
}

func (sm *SortedMempool) PopMax() *chain.Transaction {
	if sm.Len() == 0 {
		return nil
	}
	tx := sm.maxHeap.items[0].tx
	sm.Remove(tx.ID())
	return tx
}

func (sm *SortedMempool) Has(tx ids.ID) bool {
	return sm.minHeap.HasID(tx)
}

func (sm *SortedMempool) Len() int {
	return sm.minHeap.Len()
}

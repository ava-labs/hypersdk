// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSortedMempoolNew(t *testing.T) {
	// Creates empty min and max heaps
	require := require.New(t)
	sortedMempool := NewSortedMempool(0, func(tx Item) uint64 { return tx.UnitPrice() })
	require.Equal(sortedMempool.minHeap.Len(), 0, "MinHeap not initialized correctly")
	require.Equal(sortedMempool.maxHeap.Len(), 0, "MaxHeap not initialized correctly")
}

func TestSortedMempoolAdd(t *testing.T) {
	// Adds to the mempool.
	require := require.New(t)
	sortedMempool := NewSortedMempool(0, func(tx Item) uint64 { return tx.UnitPrice() })
	mempoolItem := GenerateTestItem("payer", 1, 10)
	sortedMempool.Add(mempoolItem)
	require.Equal(sortedMempool.minHeap.Len(), 1, "MaxHeap not pushed correctly")
	require.Equal(sortedMempool.maxHeap.Len(), 1, "MaxHeap not pushed correctly")
	require.True(sortedMempool.minHeap.HasID(mempoolItem.ID()), "MinHeap does not have ID")
	require.True(sortedMempool.maxHeap.HasID(mempoolItem.ID()), "MaxHeap does not have ID")
}

func TestSortedMempoolRemove(t *testing.T) {
	// Removes from the mempool.
	require := require.New(t)
	sortedMempool := NewSortedMempool(0, func(tx Item) uint64 { return tx.UnitPrice() })
	mempoolItem := GenerateTestItem("payer", 1, 10)
	// Add first
	sortedMempool.Add(mempoolItem)
	require.Equal(sortedMempool.minHeap.Len(), 1, "MaxHeap not pushed correctly")
	require.Equal(sortedMempool.maxHeap.Len(), 1, "MaxHeap not pushed correctly")
	require.True(sortedMempool.minHeap.HasID(mempoolItem.ID()), "MinHeap does not have ID")
	require.True(sortedMempool.maxHeap.HasID(mempoolItem.ID()), "MaxHeap does not have ID")
	// Remove
	sortedMempool.Remove(mempoolItem.ID())
	require.Equal(sortedMempool.minHeap.Len(), 0, "MaxHeap not removed.")
	require.Equal(sortedMempool.maxHeap.Len(), 0, "MaxHeap not removed.")
	require.False(sortedMempool.minHeap.HasID(mempoolItem.ID()), "MinHeap still has ID")
	require.False(sortedMempool.maxHeap.HasID(mempoolItem.ID()), "MaxHeap still has ID")
}

func TestSortedMempoolRemoveEmpty(t *testing.T) {
	// Try to remove a non existing entry.
	// Removes from the mempool.
	require := require.New(t)
	sortedMempool := NewSortedMempool(0, func(tx Item) uint64 { return tx.UnitPrice() })
	mempoolItem := GenerateTestItem("payer", 1, 10)
	// Require this returns
	sortedMempool.Remove(mempoolItem.ID())
	require.True(true, "not true")
}

func TestSetMinVal(t *testing.T) {
	require := require.New(t)
	payer := "payer"
	sortedMempool := NewSortedMempool(0, func(tx Item) uint64 { return tx.UnitPrice() })
	for i := uint64(0); i <= 9; i++ {
		item := GenerateTestItem(payer, 1, i)
		sortedMempool.Add(item)
		require.True(sortedMempool.Has(item.ID()), "TX not included")
	}
	// Remove half
	removed := sortedMempool.SetMinVal(5)
	require.Equal(5, len(removed), "Returned an incorrect number of txs.")
	// All timestamps less than 5
	seen := make(map[uint64]bool)
	for _, item := range removed {
		require.True(item.UnitPrice() < 5)
		_, ok := seen[item.UnitPrice()]
		require.False(ok, "Incorrect item removed.")
		seen[item.UnitPrice()] = true
	}
	// Mempool has same length
	require.Equal(5, sortedMempool.Len(), "Mempool has incorrect number of txs.")
}

func TestSetMinValRemovesAll(t *testing.T) {
	require := require.New(t)
	payer := "payer"
	sortedMempool := NewSortedMempool(0, func(tx Item) uint64 { return tx.UnitPrice() })
	var items []Item
	for i := uint64(0); i <= 4; i++ {
		item := GenerateTestItem(payer, 1, i)
		items = append(items, item)
		sortedMempool.Add(item)
		require.True(sortedMempool.Has(item.ID()), "TX not included")
	}
	// Remove more than exists
	removed := sortedMempool.SetMinVal(10)
	require.Equal(5, len(removed), "Returned an incorrect number of txs.")
	require.Equal(0, sortedMempool.Len(), "Mempool has incorrect number of txs.")
	require.Equal(items, removed, "Removed items are not as expected.")
}

func TestPeekMin(t *testing.T) {
	require := require.New(t)
	sortedMempool := NewSortedMempool(0, func(tx Item) uint64 { return tx.UnitPrice() })

	itemMin := GenerateTestItem(testPayer, 1, 1)
	itemMed := GenerateTestItem(testPayer, 1, 2)
	itemMax := GenerateTestItem(testPayer, 1, 3)
	min, ok := sortedMempool.PeekMin()
	require.False(ok)
	require.Nil(min, "Peek UnitPrice is incorrect")
	// Check PeekMin
	sortedMempool.Add(itemMed)
	require.True(sortedMempool.Has(itemMed.ID()), "TX not included")
	min, ok = sortedMempool.PeekMin()
	require.True(ok)
	require.Equal(itemMed, min, "Peek value is incorrect")

	sortedMempool.Add(itemMin)
	require.True(sortedMempool.Has(itemMin.ID()), "TX not included")
	min, ok = sortedMempool.PeekMin()
	require.True(ok)
	require.Equal(itemMin, min, "Peek value is incorrect")

	sortedMempool.Add(itemMax)
	require.True(sortedMempool.Has(itemMax.ID()), "TX not included")
	min, ok = sortedMempool.PeekMin()
	require.True(ok)
	require.Equal(itemMin, min, "Peek value is incorrect")
}

func TestPeekMax(t *testing.T) {
	require := require.New(t)

	sortedMempool := NewSortedMempool(0, func(tx Item) uint64 { return tx.UnitPrice() })

	itemMin := GenerateTestItem(testPayer, 1, 1)
	itemMed := GenerateTestItem(testPayer, 1, 2)
	itemMax := GenerateTestItem(testPayer, 1, 3)
	max, ok := sortedMempool.PeekMax()
	require.False(ok)
	require.Nil(max, "Peek UnitPrice is incorrect")
	// Check PeekMin
	sortedMempool.Add(itemMed)
	require.True(sortedMempool.Has(itemMed.ID()), "TX not included")
	max, ok = sortedMempool.PeekMax()
	require.True(ok)
	require.Equal(itemMed, max, "Peek value is incorrect")

	sortedMempool.Add(itemMin)
	require.True(sortedMempool.Has(itemMin.ID()), "TX not included")
	max, ok = sortedMempool.PeekMax()
	require.True(ok)
	require.Equal(itemMed, max, "Peek value is incorrect")

	sortedMempool.Add(itemMax)
	require.True(sortedMempool.Has(itemMax.ID()), "TX not included")
	max, ok = sortedMempool.PeekMax()
	require.True(ok)
	require.Equal(itemMax, max, "Peek value is incorrect")
}

func TestPopMin(t *testing.T) {
	require := require.New(t)

	sortedMempool := NewSortedMempool(0, func(tx Item) uint64 { return tx.UnitPrice() })

	itemMin := GenerateTestItem(testPayer, 1, 1)
	itemMed := GenerateTestItem(testPayer, 1, 2)
	itemMax := GenerateTestItem(testPayer, 1, 3)
	min, ok := sortedMempool.PopMin()
	require.False(ok)
	require.Nil(min, "Pop value is incorrect")
	// Check PeekMin
	sortedMempool.Add(itemMed)
	sortedMempool.Add(itemMin)
	sortedMempool.Add(itemMax)
	min, ok = sortedMempool.PopMin()
	require.True(ok)
	require.Equal(itemMin, min, "PopMin value is incorrect")
	min, ok = sortedMempool.PopMin()
	require.True(ok)
	require.Equal(itemMed, min, "PopMin value is incorrect")
	min, ok = sortedMempool.PopMin()
	require.True(ok)
	require.Equal(itemMax, min, "PopMin value is incorrect")
}

func TestPopMax(t *testing.T) {
	require := require.New(t)

	sortedMempool := NewSortedMempool(0, func(tx Item) uint64 { return tx.UnitPrice() })

	itemMin := GenerateTestItem(testPayer, 1, 1)
	itemMed := GenerateTestItem(testPayer, 1, 2)
	itemMax := GenerateTestItem(testPayer, 1, 3)
	max, ok := sortedMempool.PopMax()
	require.False(ok)
	require.Nil(max, "Pop value is incorrect")
	// Check PeekMin
	sortedMempool.Add(itemMed)
	sortedMempool.Add(itemMin)
	sortedMempool.Add(itemMax)

	max, ok = sortedMempool.PopMax()
	require.True(ok)
	require.Equal(itemMax, max, "PopMin value is incorrect")
	max, ok = sortedMempool.PopMax()
	require.True(ok)
	require.Equal(itemMed, max, "PopMin value is incorrect")
	max, ok = sortedMempool.PopMax()
	require.True(ok)
	require.Equal(itemMin, max, "PopMin value is incorrect")
}

func TestHas(t *testing.T) {
	require := require.New(t)

	sortedMempool := NewSortedMempool(0, func(tx Item) uint64 { return tx.UnitPrice() })
	item := GenerateTestItem(testPayer, 1, 1)
	require.False(sortedMempool.Has(item.ID()), "Found an item that was not added.")
	sortedMempool.Add(item)
	require.True(sortedMempool.Has(item.ID()), "Did not find item.")
}

func TestLen(t *testing.T) {
	require := require.New(t)

	sortedMempool := NewSortedMempool(0, func(tx Item) uint64 { return tx.UnitPrice() })
	for i := uint64(0); i <= 4; i++ {
		item := GenerateTestItem(testPayer, 1, 10)
		sortedMempool.Add(item)
		require.True(sortedMempool.Has(item.ID()), "TX not included")
	}
	// Remove more than exists
	require.Equal(5, sortedMempool.Len(), "Length of mempool is not as expected.")
}

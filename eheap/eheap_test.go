// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package eheap

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

const testSponsor = "testSponsor"

type TestItem struct {
	id        ids.ID
	sponsor   string
	timestamp int64
}

func (mti *TestItem) ID() ids.ID {
	return mti.id
}

func (mti *TestItem) Sponsor() string {
	return mti.sponsor
}

func (mti *TestItem) Expiry() int64 {
	return mti.timestamp
}

func GenerateTestItem(sponsor string, t int64) *TestItem {
	id := ids.GenerateTestID()
	return &TestItem{
		id:        id,
		sponsor:   sponsor,
		timestamp: t,
	}
}

func TestExpiryHeapNew(t *testing.T) {
	// Creates empty min and max heaps
	require := require.New(t)
	eheap := New[*TestItem](0)
	require.Zero(eheap.minHeap.Len(), "MinHeap not initialized correctly")
}

func TestExpiryHeapAdd(t *testing.T) {
	// Adds to the mempool.
	require := require.New(t)
	eheap := New[*TestItem](0)
	item := GenerateTestItem("sponsor", 1)
	eheap.Add(item)
	require.Equal(1, eheap.minHeap.Len(), "MinHeap not pushed correctly")
	require.True(eheap.minHeap.Has(item.ID()), "MinHeap does not have ID")
}

func TestExpiryHeapRemove(t *testing.T) {
	// Removes from the mempool.
	require := require.New(t)
	eheap := New[*TestItem](0)
	item := GenerateTestItem("sponsor", 1)
	// Add first
	eheap.Add(item)
	require.Equal(1, eheap.minHeap.Len(), "MinHeap not pushed correctly")
	require.True(eheap.minHeap.Has(item.ID()), "MinHeap does not have ID")
	// Remove
	eheap.Remove(item.ID())
	require.Zero(eheap.minHeap.Len(), "MinHeap not removed")
	require.False(eheap.minHeap.Has(item.ID()), "MinHeap still has ID")
}

func TestExpiryHeapRemoveEmpty(t *testing.T) {
	// Try to remove a non existing entry.
	// Removes from the mempool.
	require := require.New(t)
	eheap := New[*TestItem](0)
	item := GenerateTestItem("sponsor", 1)
	// Require this returns
	eheap.Remove(item.ID())
	require.True(true, "not true")
}

func TestSetMin(t *testing.T) {
	require := require.New(t)
	sponsor := "sponsor"
	eheap := New[*TestItem](0)
	for i := int64(0); i <= 9; i++ {
		item := GenerateTestItem(sponsor, i)
		eheap.Add(item)
		require.True(eheap.Has(item.ID()), "TX not included")
	}
	// Remove half
	removed := eheap.SetMin(5)
	require.Len(removed, 5, "Returned an incorrect number of txs.")
	// All timestamps less than 5
	seen := make(map[int64]bool)
	for _, item := range removed {
		require.Less(item.Expiry(), int64(5))
		_, ok := seen[item.Expiry()]
		require.False(ok, "Incorrect item removed.")
		seen[item.Expiry()] = true
	}
	// ExpiryHeap has same length
	require.Equal(5, eheap.Len(), "ExpiryHeap has incorrect number of txs.")
}

func TestSetMinRemovesAll(t *testing.T) {
	require := require.New(t)
	sponsor := "sponsor"
	eheap := New[*TestItem](0)
	var items []*TestItem
	for i := int64(0); i <= 4; i++ {
		item := GenerateTestItem(sponsor, i)
		items = append(items, item)
		eheap.Add(item)
		require.True(eheap.Has(item.ID()), "TX not included")
	}
	// Remove more than exists
	removed := eheap.SetMin(10)
	require.Len(removed, 5, "Returned an incorrect number of txs.")
	require.Zero(eheap.Len(), "ExpiryHeap has incorrect number of txs.")
	require.Equal(items, removed, "Removed items are not as expected.")
}

func TestPeekMin(t *testing.T) {
	require := require.New(t)
	eheap := New[*TestItem](0)

	itemMin := GenerateTestItem(testSponsor, 1)
	itemMed := GenerateTestItem(testSponsor, 2)
	itemMax := GenerateTestItem(testSponsor, 3)
	min, ok := eheap.PeekMin()
	require.False(ok)
	require.Nil(min, "Peek UnitPrice is incorrect")
	// Check PeekMin
	eheap.Add(itemMed)
	require.True(eheap.Has(itemMed.ID()), "TX not included")
	min, ok = eheap.PeekMin()
	require.True(ok)
	require.Equal(itemMed, min, "Peek value is incorrect")

	eheap.Add(itemMin)
	require.True(eheap.Has(itemMin.ID()), "TX not included")
	min, ok = eheap.PeekMin()
	require.True(ok)
	require.Equal(itemMin, min, "Peek value is incorrect")

	eheap.Add(itemMax)
	require.True(eheap.Has(itemMax.ID()), "TX not included")
	min, ok = eheap.PeekMin()
	require.True(ok)
	require.Equal(itemMin, min, "Peek value is incorrect")
}

func TestPopMin(t *testing.T) {
	require := require.New(t)

	eheap := New[*TestItem](0)

	itemMin := GenerateTestItem(testSponsor, 1)
	itemMed := GenerateTestItem(testSponsor, 2)
	itemMax := GenerateTestItem(testSponsor, 3)
	min, ok := eheap.PopMin()
	require.False(ok)
	require.Nil(min, "Pop value is incorrect")
	// Check PeekMin
	eheap.Add(itemMed)
	eheap.Add(itemMin)
	eheap.Add(itemMax)
	min, ok = eheap.PopMin()
	require.True(ok)
	require.Equal(itemMin, min, "PopMin value is incorrect")
	min, ok = eheap.PopMin()
	require.True(ok)
	require.Equal(itemMed, min, "PopMin value is incorrect")
	min, ok = eheap.PopMin()
	require.True(ok)
	require.Equal(itemMax, min, "PopMin value is incorrect")
}

func TestHas(t *testing.T) {
	require := require.New(t)

	eheap := New[*TestItem](0)
	item := GenerateTestItem(testSponsor, 1)
	require.False(eheap.Has(item.ID()), "Found an item that was not added.")
	eheap.Add(item)
	require.True(eheap.Has(item.ID()), "Did not find item.")
}

func TestLen(t *testing.T) {
	require := require.New(t)

	eheap := New[*TestItem](0)
	for i := int64(0); i <= 4; i++ {
		item := GenerateTestItem(testSponsor, i)
		eheap.Add(item)
		require.True(eheap.Has(item.ID()), "TX not included")
	}
	require.Equal(5, eheap.Len(), "Length of mempool is not as expected.")
}

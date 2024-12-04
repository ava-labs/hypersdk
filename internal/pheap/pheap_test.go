// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package pheap

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
	priority  uint64
}

func (mti *TestItem) GetID() ids.ID {
	return mti.id
}

func (mti *TestItem) Sponsor() string {
	return mti.sponsor
}

func (mti *TestItem) GetExpiry() int64 {
	return mti.timestamp
}

func (mti *TestItem) Priority() uint64 {
	return mti.priority
}

func GenerateTestItem(sponsor string, t int64, priority uint64) *TestItem {
	id := ids.GenerateTestID()
	return &TestItem{
		id:        id,
		sponsor:   sponsor,
		timestamp: t,
		priority:  priority,
	}
}

func TestPriorityHeapNew(t *testing.T) {
	// Creates empty max heaps
	require := require.New(t)
	pheap := New[*TestItem](0)
	require.Zero(pheap.maxHeap.Len(), "MaxHeap not initialized correctly")
}

func TestPriorityHeapAdd(t *testing.T) {
	// Adds to the mempool.
	require := require.New(t)
	pheap := New[*TestItem](0)
	item := GenerateTestItem("sponsor", 1, 0)
	pheap.Add(item)
	require.Equal(1, pheap.maxHeap.Len(), "MaxHeap not pushed correctly")
	require.True(pheap.maxHeap.Has(item.GetID()), "MaxHeap does not have ID")
}

func TestPriorityHeapRemove(t *testing.T) {
	// Removes from the mempool.
	require := require.New(t)
	pheap := New[*TestItem](0)
	item := GenerateTestItem("sponsor", 1, 0)
	// Add first
	pheap.Add(item)
	require.Equal(1, pheap.maxHeap.Len(), "MaxHeap not pushed correctly")
	require.True(pheap.maxHeap.Has(item.GetID()), "MaxHeap does not have ID")
	// Remove
	pheap.Remove(item.GetID())
	require.Zero(pheap.maxHeap.Len(), "MaxHeap not removed")
	require.False(pheap.maxHeap.Has(item.GetID()), "MaxHeap still has ID")
}

func TestPriorityHeapRemoveEmpty(t *testing.T) {
	// Try to remove a non existing entry.
	// Removes from the mempool.
	require := require.New(t)
	pheap := New[*TestItem](0)
	item := GenerateTestItem("sponsor", 1, 0)
	// Require this returns
	pheap.Remove(item.GetID())
	require.True(true, "not true")
}

func TestPeekFirst(t *testing.T) {
	require := require.New(t)
	pheap := New[*TestItem](0)

	itemMin := GenerateTestItem(testSponsor, 1, 1)
	itemMed := GenerateTestItem(testSponsor, 2, 2)
	itemMax := GenerateTestItem(testSponsor, 3, 3)
	max, ok := pheap.First()
	require.False(ok)
	require.Nil(max, "Peek UnitPrice is incorrect")
	// Check PeekFirst
	pheap.Add(itemMed)
	require.True(pheap.Has(itemMed.GetID()), "TX not included")
	max, ok = pheap.First()
	require.True(ok)
	require.Equal(itemMed, max, "Peek value is incorrect")

	pheap.Add(itemMax)
	require.True(pheap.Has(itemMax.GetID()), "TX not included")
	max, ok = pheap.First()
	require.True(ok)
	require.Equal(itemMax, max, "Peek value is incorrect")

	pheap.Add(itemMin)
	require.True(pheap.Has(itemMin.GetID()), "TX not included")
	max, ok = pheap.First()
	require.True(ok)
	require.Equal(itemMax, max, "Peek value is incorrect")
}

func TestPop(t *testing.T) {
	require := require.New(t)

	pheap := New[*TestItem](0)

	itemMin := GenerateTestItem(testSponsor, 1, 1)
	itemMed := GenerateTestItem(testSponsor, 2, 2)
	itemMax := GenerateTestItem(testSponsor, 3, 3)
	max, ok := pheap.Pop()
	require.False(ok)
	require.Nil(max, "Pop value is incorrect")
	// Check Pop
	pheap.Add(itemMed)
	pheap.Add(itemMin)
	pheap.Add(itemMax)
	max, ok = pheap.Pop()
	require.True(ok)
	require.Equal(itemMax, max, "PopMax value is incorrect")
	max, ok = pheap.Pop()
	require.True(ok)
	require.Equal(itemMed, max, "PopMax value is incorrect")
	max, ok = pheap.Pop()
	require.True(ok)
	require.Equal(itemMin, max, "PopMax value is incorrect")
}

func TestHas(t *testing.T) {
	require := require.New(t)

	pheap := New[*TestItem](0)
	item := GenerateTestItem(testSponsor, 1, 1)
	require.False(pheap.Has(item.GetID()), "Found an item that was not added.")
	pheap.Add(item)
	require.True(pheap.Has(item.GetID()), "Did not find item.")
}

func TestLen(t *testing.T) {
	require := require.New(t)

	pheap := New[*TestItem](0)
	for i := int64(0); i <= 4; i++ {
		item := GenerateTestItem(testSponsor, i, uint64(i))
		pheap.Add(item)
		require.True(pheap.Has(item.GetID()), "TX not included")
	}
	require.Equal(5, pheap.Len(), "Length of mempool is not as expected.")
}

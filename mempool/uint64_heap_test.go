// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"container/heap"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

const testPayer = "testPayer"

type MempoolTestItem struct {
	id        ids.ID
	payer     string
	timestamp int64
	unitPrice uint64
}

func (mti *MempoolTestItem) ID() ids.ID {
	return mti.id
}

func (mti *MempoolTestItem) Payer() string {
	return mti.payer
}

func (mti *MempoolTestItem) UnitPrice() uint64 {
	return mti.unitPrice
}

func (mti *MempoolTestItem) Expiry() int64 {
	return mti.timestamp
}

func GenerateTestItem(payer string, t int64, unitPrice uint64) *MempoolTestItem {
	id := ids.GenerateTestID()
	return &MempoolTestItem{
		id:        id,
		payer:     payer,
		timestamp: t,
		unitPrice: unitPrice,
	}
}

func TestUnit64HeapPushPopMin(t *testing.T) {
	require := require.New(t)
	minHeap := newUint64Heap[*MempoolTestItem](0, true)
	require.Equal(minHeap.Len(), 0, "heap not initialized properly.")
	mempoolItem1 := GenerateTestItem(testPayer, 1, 10)
	mempoolItem2 := GenerateTestItem(testPayer, 2, 7)
	mempoolItem3 := GenerateTestItem(testPayer, 3, 15)

	// Middle UnitPrice
	med := &uint64Entry[*MempoolTestItem]{
		id:    mempoolItem1.ID(),
		item:  mempoolItem1,
		val:   mempoolItem1.UnitPrice(),
		index: minHeap.Len(),
	}
	// Lesser UnitPrice
	low := &uint64Entry[*MempoolTestItem]{
		id:    mempoolItem2.ID(),
		item:  mempoolItem2,
		val:   mempoolItem2.UnitPrice(),
		index: minHeap.Len(),
	}
	// Greatest UnitPrice
	high := &uint64Entry[*MempoolTestItem]{
		id:    mempoolItem3.ID(),
		item:  mempoolItem3,
		val:   mempoolItem3.UnitPrice(),
		index: minHeap.Len(),
	}
	heap.Push(minHeap, med)
	heap.Push(minHeap, low)
	heap.Push(minHeap, high)
	// Added all three
	require.Equal(minHeap.Len(), 3, "Not pushed correctly.")
	// Check if added to lookup table
	_, ok := minHeap.lookup[med.id]
	require.True(ok, "Item not found in lookup.")
	_, ok = minHeap.lookup[low.id]
	require.True(ok, "Item not found in lookup.")
	_, ok = minHeap.lookup[high.id]
	require.True(ok, "Item not found in lookup.")
	// Pop and check popped correctly. Order should be 2, 1, 3
	popped := heap.Pop(minHeap)
	require.Equal(low, popped, "Incorrect item removed.")
	popped = heap.Pop(minHeap)
	require.Equal(med, popped, "Incorrect item removed.")
	popped = heap.Pop(minHeap)
	require.Equal(high, popped, "Incorrect item removed.")
}

func TestUnit64HeapPushPopMax(t *testing.T) {
	require := require.New(t)
	maxHeap := newUint64Heap[*MempoolTestItem](0, false)
	require.Equal(maxHeap.Len(), 0, "heap not initialized properly.")

	mempoolItem1 := GenerateTestItem(testPayer, 1, 10)
	mempoolItem2 := GenerateTestItem(testPayer, 2, 7)
	mempoolItem3 := GenerateTestItem(testPayer, 3, 15)

	// Middle UnitPrice
	med := &uint64Entry[*MempoolTestItem]{
		id:    mempoolItem1.ID(),
		item:  mempoolItem1,
		val:   mempoolItem1.UnitPrice(),
		index: maxHeap.Len(),
	}
	// Lesser UnitPrice
	low := &uint64Entry[*MempoolTestItem]{
		id:    mempoolItem2.ID(),
		item:  mempoolItem2,
		val:   mempoolItem2.UnitPrice(),
		index: maxHeap.Len(),
	}
	// Greatest UnitPrice
	high := &uint64Entry[*MempoolTestItem]{
		id:    mempoolItem3.ID(),
		item:  mempoolItem3,
		val:   mempoolItem3.UnitPrice(),
		index: maxHeap.Len(),
	}
	heap.Push(maxHeap, med)
	heap.Push(maxHeap, low)
	heap.Push(maxHeap, high)
	// Added all three
	require.Equal(maxHeap.Len(), 3, "Not pushed correctly.")
	// Check if added to lookup table
	_, ok := maxHeap.lookup[med.id]
	require.True(ok, "Item not found in lookup.")
	_, ok = maxHeap.lookup[low.id]
	require.True(ok, "Item not found in lookup.")
	_, ok = maxHeap.lookup[high.id]
	require.True(ok, "Item not found in lookup.")
	// Pop and check popped correctly. Order should be 2, 1, 3
	popped := heap.Pop(maxHeap)
	require.Equal(high, popped, "Incorrect item removed.")
	popped = heap.Pop(maxHeap)
	require.Equal(med, popped, "Incorrect item removed.")
	popped = heap.Pop(maxHeap)
	require.Equal(low, popped, "Incorrect item removed.")
}

func TestUnit64HeapPushExists(t *testing.T) {
	// Push an item already in heap
	require := require.New(t)
	minHeap := newUint64Heap[*MempoolTestItem](0, true)
	require.Equal(minHeap.Len(), 0, "heap not initialized properly.")
	mempoolItem := GenerateTestItem(testPayer, 1, 10)
	entry := &uint64Entry[*MempoolTestItem]{
		id:    mempoolItem.ID(),
		item:  mempoolItem,
		val:   mempoolItem.UnitPrice(),
		index: minHeap.Len(),
	}
	heap.Push(minHeap, entry)
	// Pushed correctly
	require.Equal(minHeap.Len(), 1, "Not pushed correctly.")
	// Check if added to lookup table
	_, ok := minHeap.lookup[entry.id]
	require.True(ok, "Item not found in lookup.")
	heap.Push(minHeap, entry)
	// Only 1 item
	require.Equal(minHeap.Len(), 1, "Not pushed correctly.")
}

func TestUnit64HeapGetID(t *testing.T) {
	// Push an item and grab its ID
	require := require.New(t)
	minHeap := newUint64Heap[*MempoolTestItem](0, true)
	require.Equal(minHeap.Len(), 0, "heap not initialized properly.")

	mempoolItem := GenerateTestItem(testPayer, 1, 10)
	entry := &uint64Entry[*MempoolTestItem]{
		id:    mempoolItem.ID(),
		item:  mempoolItem,
		val:   mempoolItem.UnitPrice(),
		index: minHeap.Len(),
	}
	_, ok := minHeap.GetID(mempoolItem.ID())
	require.False(ok, "Entry returned before pushing.")
	heap.Push(minHeap, entry)
	// Pushed correctly
	require.Equal(minHeap.Len(), 1, "Not pushed correctly.")
	entryReturned, ok := minHeap.GetID(mempoolItem.ID())
	require.True(ok, "Entry not returned.")
	require.Equal(entry, entryReturned, "Returned incorrect entry")
}

func TestUnit64HeapHasID(t *testing.T) {
	require := require.New(t)
	minHeap := newUint64Heap[*MempoolTestItem](0, true)
	require.Equal(minHeap.Len(), 0, "heap not initialized properly.")
	mempoolItem := GenerateTestItem(testPayer, 1, 10)
	entry := &uint64Entry[*MempoolTestItem]{
		id:    mempoolItem.ID(),
		item:  mempoolItem,
		val:   mempoolItem.UnitPrice(),
		index: minHeap.Len(),
	}
	ok := minHeap.HasID(mempoolItem.ID())
	require.False(ok, "Entry has ID before pushing.")
	heap.Push(minHeap, entry)
	// Pushed correctly
	require.Equal(minHeap.Len(), 1, "Not pushed correctly.")
	ok = minHeap.HasID(mempoolItem.ID())
	require.True(ok, "Entry was not found in heap.")
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package heap

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

type testItem struct {
	id    ids.ID
	value uint64
}

func TestUnit64HeapPushPopMin(t *testing.T) {
	require := require.New(t)
	minHeap := New[*testItem, uint64](0, true)
	require.Equal(minHeap.Len(), 0, "heap not initialized properly.")
	mempoolItem1 := &testItem{ids.GenerateTestID(), 10}
	mempoolItem2 := &testItem{ids.GenerateTestID(), 7}
	mempoolItem3 := &testItem{ids.GenerateTestID(), 15}

	// Middle UnitPrice
	med := &Entry[*testItem, uint64]{
		ID:    mempoolItem1.id,
		Item:  mempoolItem1,
		Val:   mempoolItem1.value,
		Index: minHeap.Len(),
	}
	// Lesser UnitPrice
	low := &Entry[*testItem, uint64]{
		ID:    mempoolItem2.id,
		Item:  mempoolItem2,
		Val:   mempoolItem2.value,
		Index: minHeap.Len(),
	}
	// Greatest UnitPrice
	high := &Entry[*testItem, uint64]{
		ID:    mempoolItem3.id,
		Item:  mempoolItem3,
		Val:   mempoolItem3.value,
		Index: minHeap.Len(),
	}
	minHeap.Add(med)
	minHeap.Add(low)
	minHeap.Add(high)
	// Added all three
	require.Equal(minHeap.Len(), 3, "Not pushed correctly.")
	// Check if added to lookup table
	_, ok := minHeap.lookup[med.ID]
	require.True(ok, "Item not found in lookup.")
	_, ok = minHeap.lookup[low.ID]
	require.True(ok, "Item not found in lookup.")
	_, ok = minHeap.lookup[high.ID]
	require.True(ok, "Item not found in lookup.")
	// Pop and check popped correctly. Order should be 2, 1, 3
	popped := minHeap.Remove()
	require.Equal(low, popped, "Incorrect item removed.")
	popped = minHeap.Remove()
	require.Equal(med, popped, "Incorrect item removed.")
	popped = minHeap.Remove()
	require.Equal(high, popped, "Incorrect item removed.")
}

func TestUnit64HeapPushPopMax(t *testing.T) {
	require := require.New(t)
	maxHeap := New[*testItem, uint64](0, false)
	require.Equal(maxHeap.Len(), 0, "heap not initialized properly.")

	mempoolItem1 := &testItem{ids.GenerateTestID(), 10}
	mempoolItem2 := &testItem{ids.GenerateTestID(), 7}
	mempoolItem3 := &testItem{ids.GenerateTestID(), 15}

	// Middle UnitPrice
	med := &Entry[*testItem, uint64]{
		ID:    mempoolItem1.id,
		Item:  mempoolItem1,
		Val:   mempoolItem1.value,
		Index: maxHeap.Len(),
	}
	// Lesser UnitPrice
	low := &Entry[*testItem, uint64]{
		ID:    mempoolItem2.id,
		Item:  mempoolItem2,
		Val:   mempoolItem2.value,
		Index: maxHeap.Len(),
	}
	// Greatest UnitPrice
	high := &Entry[*testItem, uint64]{
		ID:    mempoolItem3.id,
		Item:  mempoolItem3,
		Val:   mempoolItem3.value,
		Index: maxHeap.Len(),
	}
	maxHeap.Add(med)
	maxHeap.Add(low)
	maxHeap.Add(high)
	// Added all three
	require.Equal(maxHeap.Len(), 3, "Not pushed correctly.")
	// Check if added to lookup table
	_, ok := maxHeap.lookup[med.ID]
	require.True(ok, "Item not found in lookup.")
	_, ok = maxHeap.lookup[low.ID]
	require.True(ok, "Item not found in lookup.")
	_, ok = maxHeap.lookup[high.ID]
	require.True(ok, "Item not found in lookup.")
	// Pop and check popped correctly. Order should be 2, 1, 3
	popped := maxHeap.Remove()
	require.Equal(high, popped, "Incorrect item removed.")
	popped = maxHeap.Remove()
	require.Equal(med, popped, "Incorrect item removed.")
	popped = maxHeap.Remove()
	require.Equal(low, popped, "Incorrect item removed.")
}

func TestUnit64HeapPushExists(t *testing.T) {
	// Push an item already in heap
	require := require.New(t)
	minHeap := New[*testItem, uint64](0, true)
	require.Equal(minHeap.Len(), 0, "heap not initialized properly.")
	mempoolItem := &testItem{ids.GenerateTestID(), 10}
	entry := &Entry[*testItem, uint64]{
		ID:    mempoolItem.id,
		Item:  mempoolItem,
		Val:   mempoolItem.value,
		Index: minHeap.Len(),
	}
	minHeap.Add(entry)
	// Pushed correctly
	require.Equal(minHeap.Len(), 1, "Not pushed correctly.")
	// Check if added to lookup table
	_, ok := minHeap.lookup[entry.ID]
	require.True(ok, "Item not found in lookup.")
	minHeap.Add(entry)
	// Only 1 item
	require.Equal(minHeap.Len(), 1, "Not pushed correctly.")
}

func TestUnit64HeapGetID(t *testing.T) {
	// Push an item and grab its ID
	require := require.New(t)
	minHeap := New[*testItem, uint64](0, true)
	require.Equal(minHeap.Len(), 0, "heap not initialized properly.")

	mempoolItem := &testItem{ids.GenerateTestID(), 10}
	entry := &Entry[*testItem, uint64]{
		ID:    mempoolItem.id,
		Item:  mempoolItem,
		Val:   mempoolItem.value,
		Index: minHeap.Len(),
	}
	_, ok := minHeap.GetID(mempoolItem.id)
	require.False(ok, "Entry returned before pushing.")
	minHeap.Add(entry)
	// Pushed correctly
	require.Equal(minHeap.Len(), 1, "Not pushed correctly.")
	entryReturned, ok := minHeap.GetID(mempoolItem.id)
	require.True(ok, "Entry not returned.")
	require.Equal(entry, entryReturned, "Returned incorrect entry")
}

func TestUnit64HeapHasID(t *testing.T) {
	require := require.New(t)
	minHeap := New[*testItem, uint64](0, true)
	require.Equal(minHeap.Len(), 0, "heap not initialized properly.")
	mempoolItem := &testItem{ids.GenerateTestID(), 10}
	entry := &Entry[*testItem, uint64]{
		ID:    mempoolItem.id,
		Item:  mempoolItem,
		Val:   mempoolItem.value,
		Index: minHeap.Len(),
	}
	ok := minHeap.HasID(mempoolItem.id)
	require.False(ok, "Entry has ID before pushing.")
	minHeap.Add(entry)
	// Pushed correctly
	require.Equal(minHeap.Len(), 1, "Not pushed correctly.")
	ok = minHeap.HasID(mempoolItem.id)
	require.True(ok, "Entry was not found in heap.")
}

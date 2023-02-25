// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package emap

import (
	"container/heap"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/stretchr/testify/require"
)

type TestTx struct {
	id ids.ID
	t  int64
}

func (tx *TestTx) ID() ids.ID    { return tx.id }
func (tx *TestTx) Expiry() int64 { return tx.t }

func TestBucketHeapLen(t *testing.T) {
	require := require.New(t)
	// Create new bucket
	bh := &bucketHeap{
		buckets: []*bucket{
			{
				t:     1,
				items: []ids.ID{},
			},
		},
	}
	require.Equal(bh.Len(), 1, "Length was not equal to one.")
}

func TestBucketHeapLess(t *testing.T) {
	require := require.New(t)
	// Create new bucket
	bh := &bucketHeap{
		buckets: []*bucket{
			{
				t:     1,
				items: []ids.ID{},
			},
			{
				t:     2,
				items: []ids.ID{},
			},
		},
	}

	require.True(bh.Less(0, 1), "Less function returned incorrectly.")
	require.False(bh.Less(1, 0), "Less function returned incorrectly.")
}

func TestBucketHeapSwap(t *testing.T) {
	require := require.New(t)
	firstBucket := bucket{
		t:     1,
		items: []ids.ID{},
	}
	secondBucket := bucket{
		t:     2,
		items: []ids.ID{},
	}
	bh := &bucketHeap{
		buckets: []*bucket{
			&firstBucket,
			&secondBucket,
		},
	}
	// Perform Swap
	bh.Swap(0, 1)
	require.Equal(bh.buckets[0], &secondBucket, "Buckets swapped incorrectly.")
	require.Equal(bh.buckets[1], &firstBucket, "Buckets swapped incorrectly.")
}

func TestBucketHeapPush(t *testing.T) {
	require := require.New(t)

	b := bucket{
		t:     1,
		items: []ids.ID{},
	}
	bh := &bucketHeap{
		buckets: []*bucket{},
	}
	// Push
	heap.Push(bh, &b)
	require.Equal(bh.buckets[0], &b, "Buckets pushed incorrectly")
}

func TestBucketHeapPushPanics(t *testing.T) {
	require := require.New(t)

	b := bucket{
		t:     1,
		items: []ids.ID{},
	}
	bh := &bucketHeap{
		buckets: []*bucket{},
	}
	// Push
	f := func() { heap.Push(bh, b) }
	require.Panics(f, "Did not panic after incorrect interface push.")
}

func TestBucketHeapPop(t *testing.T) {
	require := require.New(t)
	firstBucket := bucket{
		t:     1,
		items: []ids.ID{},
	}
	secondBucket := bucket{
		t:     2,
		items: []ids.ID{},
	}
	bh := &bucketHeap{
		buckets: []*bucket{
			&firstBucket,
			&secondBucket,
		},
	}
	popped := heap.Pop(bh)
	require.Equal(bh.Len(), 1, "Pop did not adjust length correctly.")
	require.Equal(&firstBucket, popped, "Pop did not return correct bucket.")
}

func TestBucketHeapPeek(t *testing.T) {
	require := require.New(t)
	firstBucket := bucket{
		t:     1,
		items: []ids.ID{},
	}
	secondBucket := bucket{
		t:     2,
		items: []ids.ID{},
	}
	bh := &bucketHeap{
		buckets: []*bucket{
			&firstBucket,
			&secondBucket,
		},
	}
	peeked := bh.Peek()
	require.Equal(bh.Len(), 2, "Peek changed the length of buckets.")
	require.Equal(&firstBucket, peeked, "Peek did not return correct bucket.")
}

func TestBucketHeapPeekEmpty(t *testing.T) {
	require := require.New(t)
	bh := &bucketHeap{
		buckets: []*bucket{},
	}
	peeked := bh.Peek()
	require.Nil(peeked, "Peek was not nil.")
}

func TestBucketHeapPushPop(t *testing.T) {
	// Pushes buckets in unordered fashion then checks pop returns correctly
	require := require.New(t)
	firstBucket := bucket{
		t:     1,
		items: []ids.ID{},
	}
	secondBucket := bucket{
		t:     2,
		items: []ids.ID{},
	}
	thirdBucket := bucket{
		t:     3,
		items: []ids.ID{},
	}
	bh := &bucketHeap{
		buckets: []*bucket{},
	}
	heap.Push(bh, &secondBucket)
	heap.Push(bh, &firstBucket)
	heap.Push(bh, &thirdBucket)

	require.Equal(bh.Len(), 3, "BH not pushed correctly.")
	require.Equal(&firstBucket, heap.Pop(bh), "Pop returned incorrect value")
	require.Equal(&secondBucket, heap.Pop(bh), "Pop returned incorrect value")
	require.Equal(&thirdBucket, heap.Pop(bh), "Pop returned incorrect value")
}

func TestEmapNew(t *testing.T) {
	require := require.New(t)
	e := NewEMap[*TestTx]()
	emptyE := &EMap[*TestTx]{
		seen:  set.Set[ids.ID]{},
		times: make(map[int64]*bucket),
		bh: &bucketHeap{
			buckets: []*bucket{},
		},
	}
	require.Equal(emptyE.seen, e.seen, "Emap did not return an empty emap struct.")
	require.Equal(emptyE.times, e.times, "Emap did not return an empty emap struct.")
	require.Equal(emptyE.bh, e.bh, "Emap did not return an empty emap struct.")
}

func TestEmapAddIDGenesis(t *testing.T) {
	require := require.New(t)
	e := &EMap[*TestTx]{
		seen:  set.Set[ids.ID]{},
		times: make(map[int64]*bucket),
		bh: &bucketHeap{
			buckets: []*bucket{},
		},
	}
	var timestamp int64 = 0
	id := ids.GenerateTestID()
	tx := &TestTx{
		t:  timestamp,
		id: id,
	}
	txs := []*TestTx{tx}
	e.Add(txs)
	// seen was updated
	_, okSeen := e.seen[tx.ID()]
	require.False(okSeen, "Genesis timestamp was incorrectly added")
	// get bucket
	_, okBucket := e.times[0]
	require.False(okBucket, "Genesis timestamp found in bucket")
}

func TestEmapAddIDNewBucket(t *testing.T) {
	require := require.New(t)

	e := &EMap[*TestTx]{
		seen:  set.Set[ids.ID]{},
		times: make(map[int64]*bucket),
		bh: &bucketHeap{
			buckets: []*bucket{},
		},
	}
	var timestamp int64 = 1

	id := ids.GenerateTestID()
	tx := &TestTx{
		t:  timestamp,
		id: id,
	}
	txs := []*TestTx{tx}
	e.Add(txs)

	// seen was updated
	_, okSeen := e.seen[tx.ID()]
	require.True(okSeen, "Could not find id in seen map")
	// get bucket
	b, okBucket := e.times[timestamp]
	require.True(okBucket, "Could not find time bucket")
	require.Equal(len(b.items), 1, "Bucket length is incorrect")
}

func TestEmapAddIDExists(t *testing.T) {
	require := require.New(t)

	e := &EMap[*TestTx]{
		seen:  set.Set[ids.ID]{},
		times: make(map[int64]*bucket),
		bh: &bucketHeap{
			buckets: []*bucket{},
		},
	}
	var timestamp int64 = 1
	id := ids.GenerateTestID()
	tx1 := &TestTx{
		t:  timestamp,
		id: id,
	}
	txs := []*TestTx{tx1}
	e.Add(txs)
	_, okSeen := e.seen[tx1.ID()]
	require.True(okSeen, "Could not find id in seen map")
	tx2 := &TestTx{
		t:  timestamp * 3,
		id: id,
	}
	txs = []*TestTx{tx2}
	e.Add(txs)

	// get bucket
	b, okBucket := e.times[timestamp]
	require.True(okBucket, "Could not find time bucket")
	require.Equal(len(b.items), 1, "Bucket length is incorrect")
}

func TestEmapAddIDBucketExists(t *testing.T) {
	require := require.New(t)

	e := &EMap[*TestTx]{
		seen:  set.Set[ids.ID]{},
		times: make(map[int64]*bucket),
		bh: &bucketHeap{
			buckets: []*bucket{},
		},
	}
	var timestamp int64 = 1

	id1 := ids.GenerateTestID()
	id2 := ids.GenerateTestID()

	tx1 := &TestTx{
		t:  timestamp,
		id: id1,
	}
	tx2 := &TestTx{
		t:  timestamp,
		id: id2,
	}
	txs := []*TestTx{tx1}
	e.Add(txs)
	_, okSeen := e.seen[tx1.ID()]
	require.True(okSeen, "Could not find id in seen map")
	txs = []*TestTx{tx2}
	e.Add(txs)
	// seen was updated
	_, okSeen = e.seen[tx2.ID()]
	require.True(okSeen, "Could not find id in seen map")
	// get bucket
	b, okBucket := e.times[timestamp]
	require.True(okBucket, "Could not find time bucket")
	require.Equal(len(b.items), 2, "Bucket length is incorrect")
}

func TestEmapAny(t *testing.T) {
	require := require.New(t)
	e := &EMap[*TestTx]{
		seen:  set.Set[ids.ID]{},
		times: make(map[int64]*bucket),
		bh: &bucketHeap{
			buckets: []*bucket{},
		},
	}
	var timestamp int64 = 1
	id := ids.GenerateTestID()
	tx := &TestTx{
		t:  timestamp,
		id: id,
	}
	txs := []*TestTx{tx}
	e.Add(txs)
	require.True(e.Any(txs), "Did not find transactions.")
}

func TestSetMin(t *testing.T) {
	// Sets min to timestamp 3. Requires all buckets
	// to be removed with timestamp t less than min
	require := require.New(t)
	e := &EMap[*TestTx]{
		seen:  set.Set[ids.ID]{},
		times: make(map[int64]*bucket),
		bh: &bucketHeap{
			buckets: []*bucket{},
		},
	}
	pushedIds := []ids.ID{}
	startT := int64(1)
	minT := int64(3)
	endT := int64(6)

	for n := startT; n < endT; n++ {
		id := ids.GenerateTestID()
		e.add(id, n)
		_, okSeen := e.seen[id]
		pushedIds = append(pushedIds, id)
		require.True(okSeen, "Id not set in seen list")
	}
	// Set min to 3
	removedIds := e.SetMin(minT)
	// Check removed_ids = min_ids
	require.Equal(pushedIds[:minT-1], removedIds, "Did not set the minimum correctly")
	// Check ids were removed from seen list
	for t, id := range pushedIds {
		_, okSeen := e.seen[id]
		// Convert from index to t
		if int64(t)+1 < minT {
			require.False(okSeen, "Id not removed from seen list")
		} else {
			require.True(okSeen, "Id removed from seen list")
		}
	}
	// Check that buckets were removed from bh
	for t := startT; t < minT; t++ {
		_, ok := e.times[t]
		require.False(ok, "Bucket not removed from bh")
	}
}

func TestSetMinPopsAll(t *testing.T) {
	// Sets min to be higher than all timestamps. Should remove all
	// buckets and ids in EMap.
	require := require.New(t)
	e := &EMap[*TestTx]{
		seen:  set.Set[ids.ID]{},
		times: make(map[int64]*bucket),
		bh: &bucketHeap{
			buckets: []*bucket{},
		},
	}
	pushedIds := []ids.ID{}
	startT := int64(1)
	endT := int64(6)
	minT := int64(10)

	for n := startT; n < endT; n++ {
		id := ids.GenerateTestID()
		e.add(id, n)
		_, okSeen := e.seen[id]
		pushedIds = append(pushedIds, id)
		require.True(okSeen, "Id not set in seen list")
	}
	removedIds := e.SetMin(minT)
	// Check removed_ids = min_ids
	require.Equal(pushedIds, removedIds, "Not all ids were returned")
	// Check EMap is empty
	emptyEmap := &EMap[*TestTx]{
		seen:  set.Set[ids.ID]{},
		times: make(map[int64]*bucket),
		bh: &bucketHeap{
			buckets: []*bucket{},
		},
	}
	require.Equal(emptyEmap, e, "EMap not empty")
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package emap

import (
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/heap"
	"github.com/stretchr/testify/require"
)

type TestTx struct {
	id ids.ID
	t  int64
}

func (tx *TestTx) ID() ids.ID    { return tx.id }
func (tx *TestTx) Expiry() int64 { return tx.t }

func TestEmapNew(t *testing.T) {
	require := require.New(t)
	e := NewEMap[*TestTx]()
	emptyE := &EMap[*TestTx]{
		seen:  set.Set[ids.ID]{},
		times: make(map[int64]*bucket),
		bh:    heap.New[*bucket, int64](0, true),
	}
	require.Equal(emptyE.seen, e.seen, "Emap did not return an empty emap struct.")
	require.Equal(emptyE.times, e.times, "Emap did not return an empty emap struct.")
	require.Equal(emptyE.bh, e.bh, "Emap did not return an empty emap struct.")
}

func TestEmapAddIDGenesis(t *testing.T) {
	require := require.New(t)
	e := NewEMap[*TestTx]()
	var timestamp int64 = 0
	id := ids.GenerateTestID()
	tx := &TestTx{
		t:  timestamp,
		id: id,
	}
	txs := []*TestTx{tx}
	e.Add(txs)
	// Seen was updated
	_, okSeen := e.seen[tx.ID()]
	require.False(okSeen, "Genesis timestamp was incorrectly added")
	// Get bucket
	_, okBucket := e.times[0]
	require.False(okBucket, "Genesis timestamp found in bucket")

	// Check bucket heap was not updated
	require.False(e.bh.Has(id), "BH has ID")
}

func TestEmapAddIDNewBucket(t *testing.T) {
	require := require.New(t)

	e := NewEMap[*TestTx]()
	timestamp := time.Now().UnixMilli()

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
	b, okBucket := e.times[reducePrecision(timestamp)]
	require.True(okBucket, "Could not find time bucket")
	require.Equal(len(b.items), 1, "Bucket length is incorrect")

	// Check bucket heap was updated
	require.True(e.bh.Has(id), "BH does not have ID")
}

func TestEmapAddIDExists(t *testing.T) {
	require := require.New(t)
	e := NewEMap[*TestTx]()

	timestamp := time.Now().UnixMilli()
	id := ids.GenerateTestID()
	tx1 := &TestTx{
		t:  timestamp,
		id: id,
	}
	txs := []*TestTx{tx1}
	e.Add(txs)
	_, okSeen := e.seen[tx1.ID()]
	require.True(okSeen, "Could not find id in seen map")

	entry, ok := e.bh.Get(id)
	// Check bh
	require.True(ok, "BH does not have ID")
	require.Equal(entry.Val, reducePrecision(tx1.t), "BH incorrectly set val.")

	tx2 := &TestTx{
		t:  timestamp * 3,
		id: id,
	}
	txs = []*TestTx{tx2}
	e.Add(txs)

	// get bucket
	b, okBucket := e.times[reducePrecision(timestamp)]
	require.True(okBucket, "Could not find time bucket")
	require.Equal(len(b.items), 1, "Bucket length is incorrect")

	entry, ok = e.bh.Get(id)
	// Check bh
	require.True(ok, "BH does not have ID")
	require.Equal(entry.Val, reducePrecision(tx1.t), "BH incorrectly updated.")
}

func TestEmapAddIDBucketExists(t *testing.T) {
	require := require.New(t)

	e := NewEMap[*TestTx]()

	timestamp := time.Now().UnixMilli()

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
	b, okBucket := e.times[reducePrecision(timestamp)]
	require.True(okBucket, "Could not find time bucket")
	require.Equal(1, e.bh.Len(), "Number of buckets is incorrect.")
	require.Equal(len(b.items), 2, "Bucket length is incorrect")

	entry, ok := e.bh.Get(id1)
	// Check bh
	require.True(ok, "BH does not have ID")
	require.Equal([]ids.ID{id1, id2}, entry.Item.items, "BH incorrectly updated.")
}

func TestEmapAny(t *testing.T) {
	require := require.New(t)
	e := NewEMap[*TestTx]()

	timestamp := time.Now().UnixMilli()
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
	// Sets min to timestamp now + 2s. Requires all buckets
	// to be removed with timestamp t less than min.
	require := require.New(t)
	e := NewEMap[*TestTx]()

	pushedIds := []ids.ID{}
	startT := time.Now().UnixMilli()
	endT := startT + 5000 // 5s
	minT := startT + 2000 // 2s
	for n := startT; n < endT; n++ {
		id := ids.GenerateTestID()
		e.add(id, n)
		_, okSeen := e.seen[id]
		pushedIds = append(pushedIds, id)
		require.True(okSeen, "Id not set in seen list")
		require.True(e.bh.Has(id), "ID not added to bh")
	}
	// Check bh
	require.Equal(len(pushedIds), e.bh.Len(), "BH not added to properly")

	// Set min
	removedIds := e.SetMin(minT)
	// Check removed_ids = min_ids
	removedIndex := reducePrecision(minT) - startT
	require.EqualValues(pushedIds[:removedIndex], removedIds, "Did not set the minimum correctly")
	// Check ids were removed from seen list
	for i, id := range pushedIds {
		_, okSeen := e.seen[id]
		// Convert from index to t
		if int64(i) < removedIndex {
			require.False(okSeen, "Id not removed from seen list")
		} else {
			require.True(okSeen, "Id removed from seen list")
		}
	}
	// Check that buckets were removed from bh
	i := 0
	for t := startT; t < reducePrecision(minT); t++ {
		_, ok := e.times[reducePrecision(t)]
		require.False(ok, "Bucket not removed from bh")
		require.False(e.bh.Has(pushedIds[i]), "Bucket not removed from bh")
		i++
	}
	require.Equal(int(endT-reducePrecision(minT)), e.bh.Len(), "Items not removed from bh")
}

func TestSetMinPopsAll(t *testing.T) {
	// Sets min to be higher than all timestamps. Should remove all
	// buckets and ids in EMap.
	require := require.New(t)
	e := NewEMap[*TestTx]()

	pushedIds := []ids.ID{}
	startT := time.Now().UnixMilli()
	endT := startT + 6000   // 6s
	minT := startT + 10_000 // 10s

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
	emptyEmap := NewEMap[*TestTx]()

	require.Equal(emptyEmap, e, "EMap not empty")
}

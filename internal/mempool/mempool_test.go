// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
)

var testSponsor = codec.CreateAddress(1, ids.GenerateTestID())

type TestItem struct {
	id        ids.ID
	sponsor   codec.Address
	timestamp int64
}

func (mti *TestItem) GetID() ids.ID {
	return mti.id
}

func (mti *TestItem) GetSponsor() codec.Address {
	return mti.sponsor
}

func (mti *TestItem) GetExpiry() int64 {
	return mti.timestamp
}

func (*TestItem) Size() int {
	return 2 // distinguish from len
}

func GenerateTestItem(sponsor codec.Address, t int64) *TestItem {
	id := ids.GenerateTestID()
	return &TestItem{
		id:        id,
		sponsor:   sponsor,
		timestamp: t,
	}
}

func TestMempool(t *testing.T) {
	require := require.New(t)

	ctx := context.TODO()
	txm := New[*TestItem](trace.Noop, 3, 16)

	for _, i := range []int64{100, 200, 300, 400} {
		item := GenerateTestItem(testSponsor, i)
		items := []*TestItem{item}
		txm.Add(ctx, items)
	}
	next, ok := txm.PeekNext(ctx)
	require.True(ok)
	require.Equal(int64(100), next.GetExpiry())
	require.Equal(3, txm.Len(ctx))
	require.Equal(6, txm.Size(ctx))
}

func TestMempoolAddDuplicates(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()
	txm := New[*TestItem](trace.Noop, 3, 16)
	// Generate item
	item := GenerateTestItem(testSponsor, 300)
	items := []*TestItem{item}
	txm.Add(ctx, items)
	require.Equal(1, txm.Len(ctx), "Item not added.")
	next, ok := txm.PeekNext(ctx)
	require.True(ok)
	require.Equal(int64(300), next.GetExpiry())
	// Add again
	txm.Add(ctx, items)
	require.Equal(1, txm.Len(ctx), "Item not added.")
}

func TestMempoolAddExceedMaxSponsorSize(t *testing.T) {
	// Sponsor1 has reached his max
	// Sponsor2 is exempt from max size
	require := require.New(t)
	ctx := context.TODO()
	sponsor := codec.CreateAddress(4, ids.GenerateTestID())
	// Non exempt sponsors max of 4
	txm := New[*TestItem](trace.Noop, 20, 4)
	// Add 6 transactions for each sponsor
	for i := int64(0); i <= 5; i++ {
		itemSponsor := GenerateTestItem(sponsor, i)
		txm.Add(ctx, []*TestItem{itemSponsor})
	}
	require.Equal(4, txm.Len(ctx), "Mempool has incorrect txs.")
	require.Equal(4, txm.owned[sponsor], "Sponsor has incorrect txs.")
}

func TestMempoolAddExceedMaxSize(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()

	txm := New[*TestItem](trace.Noop, 3, 20)
	// Add more tx's than txm.maxSize
	for i := int64(0); i < 10; i++ {
		item := GenerateTestItem(testSponsor, i)
		items := []*TestItem{item}
		txm.Add(ctx, items)
		if i < 3 {
			require.True(txm.Has(ctx, item.GetID()), "TX not included")
		} else {
			require.False(txm.Has(ctx, item.GetID()), "TX included")
		}
	}
	// Pop and check values
	for i := int64(0); i < 3; i++ {
		popped, ok := txm.PopNext(ctx)
		require.True(ok)
		require.Equal(i, popped.GetExpiry(), "Mempool did not pop correct tx.")
	}
	_, ok := txm.owned[testSponsor]
	require.False(ok, "Sponsor not removed from owned.")
	require.Equal(0, txm.Len(ctx), "Mempool has incorrect number of txs.")
}

func TestMempoolRemoveTxs(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()

	txm := New[*TestItem](trace.Noop, 3, 20)
	// Add
	item := GenerateTestItem(testSponsor, 10)
	items := []*TestItem{item}
	txm.Add(ctx, items)
	require.True(txm.Has(ctx, item.GetID()), "TX not included")
	// Remove
	itemNotIn := GenerateTestItem(testSponsor, 10)
	items = []*TestItem{item, itemNotIn}
	txm.Remove(ctx, items)
	require.Equal(0, txm.Len(ctx), "Mempool has incorrect number of txs.")
}

func TestMempoolSetMinTimestamp(t *testing.T) {
	require := require.New(t)
	ctx := context.TODO()

	txm := New[*TestItem](trace.Noop, 20, 20)
	// Add more tx's than txm.maxSize
	for i := int64(0); i < 10; i++ {
		item := GenerateTestItem(testSponsor, i)
		items := []*TestItem{item}
		txm.Add(ctx, items)
		require.True(txm.Has(ctx, item.GetID()), "TX not included")
	}
	// Remove half
	removed := txm.SetMinTimestamp(ctx, 5)
	require.Len(removed, 5, "Mempool has incorrect number of txs.")
	// All timestamps less than 5
	seen := make(map[int64]bool)
	for _, item := range removed {
		require.Less(item.GetExpiry(), int64(5))
		_, ok := seen[item.GetExpiry()]
		require.False(ok)
		seen[item.GetExpiry()] = true
	}
	// Mempool has same length
	require.Equal(5, txm.Len(ctx), "Mempool has incorrect number of txs.")
}

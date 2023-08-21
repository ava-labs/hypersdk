// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/trace"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

const testPayer = "testPayer"

type TestItem struct {
	id        ids.ID
	payer     string
	timestamp int64
}

func (mti *TestItem) ID() ids.ID {
	return mti.id
}

func (mti *TestItem) Payer() string {
	return mti.payer
}

func (mti *TestItem) Expiry() int64 {
	return mti.timestamp
}

func GenerateTestItem(payer string, t int64) *TestItem {
	id := ids.GenerateTestID()
	return &TestItem{
		id:        id,
		payer:     payer,
		timestamp: t,
	}
}

func TestMempool(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.TODO()
	tracer, _ := trace.New(&trace.Config{Enabled: false})
	txm := New[*TestItem](tracer, 3, 16, nil)

	for _, i := range []int64{100, 200, 300, 400} {
		item := GenerateTestItem(testPayer, i)
		items := []*TestItem{item}
		txm.Add(ctx, items)
	}
	next, ok := txm.PeekNext(ctx)
	require.True(ok)
	require.Equal(int64(100), next.Expiry())
	require.Equal(3, txm.Len(ctx))
}

func TestMempoolAddDuplicates(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.TODO()
	tracer, _ := trace.New(&trace.Config{Enabled: false})
	txm := New[*TestItem](tracer, 3, 16, nil)
	// Generate item
	item := GenerateTestItem(testPayer, 300)
	items := []*TestItem{item}
	txm.Add(ctx, items)
	require.Equal(1, txm.Len(ctx), "Item not added.")
	next, ok := txm.PeekNext(ctx)
	require.True(ok)
	require.Equal(int64(300), next.Expiry())
	// Add again
	txm.Add(ctx, items)
	require.Equal(1, txm.Len(ctx), "Item not added.")
}

func TestMempoolAddExceedMaxPayerSize(t *testing.T) {
	// Payer1 has reached his max
	// Payer2 is exempt from max size
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.TODO()
	tracer, _ := trace.New(&trace.Config{Enabled: false})
	exemptPayer := "IAMEXEMPT"
	payer := "notexempt"
	exemptPayers := []byte(exemptPayer)
	// Non exempt payers max of 4
	txm := New[*TestItem](tracer, 20, 4, [][]byte{exemptPayers})
	// Add 6 transactions for each payer
	for i := int64(0); i <= 5; i++ {
		itemPayer := GenerateTestItem(payer, i)
		itemExempt := GenerateTestItem(exemptPayer, i)
		items := []*TestItem{itemPayer, itemExempt}
		txm.Add(ctx, items)
	}
	require.Equal(10, txm.Len(ctx), "Mempool has incorrect txs.")
	require.Equal(4, txm.owned[payer], "Payer has incorrect txs.")
	require.Equal(6, txm.owned[exemptPayer], "Payer has incorrect txs.")
}

func TestMempoolAddExceedMaxSize(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.TODO()
	tracer, _ := trace.New(&trace.Config{Enabled: false})

	txm := New[*TestItem](tracer, 3, 20, nil)
	// Add more tx's than txm.maxSize
	for i := int64(0); i < 10; i++ {
		item := GenerateTestItem(testPayer, i)
		items := []*TestItem{item}
		txm.Add(ctx, items)
		if i < 3 {
			require.True(txm.Has(ctx, item.ID()), "TX not included")
		} else {
			require.False(txm.Has(ctx, item.ID()), "TX included")
		}
	}
	// Pop and check values
	for i := int64(0); i < 3; i++ {
		popped, ok := txm.PopNext(ctx)
		require.True(ok)
		require.Equal(i, popped.Expiry(), "Mempool did not pop correct tx.")
	}
	_, ok := txm.owned[testPayer]
	require.False(ok, "Payer not removed from owned.")
	require.Equal(0, txm.Len(ctx), "Mempool has incorrect number of txs.")
}

func TestMempoolRemoveTxs(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.TODO()
	tracer, _ := trace.New(&trace.Config{Enabled: false})

	txm := New[*TestItem](tracer, 3, 20, nil)
	// Add
	item := GenerateTestItem(testPayer, 10)
	items := []*TestItem{item}
	txm.Add(ctx, items)
	require.True(txm.Has(ctx, item.ID()), "TX not included")
	// Remove
	itemNotIn := GenerateTestItem(testPayer, 10)
	items = []*TestItem{item, itemNotIn}
	txm.Remove(ctx, items)
	require.Equal(0, txm.Len(ctx), "Mempool has incorrect number of txs.")
}

func TestMempoolSetMinTimestamp(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.TODO()
	tracer, _ := trace.New(&trace.Config{Enabled: false})

	txm := New[*TestItem](tracer, 20, 20, nil)
	// Add more tx's than txm.maxSize
	for i := int64(0); i < 10; i++ {
		item := GenerateTestItem(testPayer, i)
		items := []*TestItem{item}
		txm.Add(ctx, items)
		require.True(txm.Has(ctx, item.ID()), "TX not included")
	}
	// Remove half
	removed := txm.SetMinTimestamp(ctx, 5)
	require.Equal(5, len(removed), "Mempool has incorrect number of txs.")
	// All timestamps less than 5
	seen := make(map[int64]bool)
	for _, item := range removed {
		require.True(item.Expiry() < 5)
		_, ok := seen[item.Expiry()]
		require.False(ok)
		seen[item.Expiry()] = true
	}
	// Mempool has same length
	require.Equal(5, txm.Len(ctx), "Mempool has incorrect number of txs.")
}

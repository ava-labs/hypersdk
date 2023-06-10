// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package mempool

import (
	"context"
	"testing"

	"github.com/AnomalyFi/hypersdk/trace"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestMempool(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	ctx := context.TODO()
	tracer, _ := trace.New(&trace.Config{Enabled: false})
	txm := New[*MempoolTestItem](tracer, 3, 16, nil)

	for _, i := range []uint64{100, 200, 300, 400} {
		item := GenerateTestItem(testPayer, 1, i)
		items := []*MempoolTestItem{item}
		txm.Add(ctx, items)
	}
	max, ok := txm.PeekMax(ctx)
	require.True(ok)
	require.Equal(uint64(400), max.UnitPrice())
	min, ok := txm.PeekMin(ctx)
	require.True(ok)
	require.Equal(uint64(200), min.UnitPrice())
	require.Equal(3, txm.Len(ctx))
}

func TestMempoolAddDuplicates(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.TODO()
	tracer, _ := trace.New(&trace.Config{Enabled: false})
	txm := New[*MempoolTestItem](tracer, 3, 16, nil)
	// Generate item
	item := GenerateTestItem(testPayer, 1, 300)
	items := []*MempoolTestItem{item}
	txm.Add(ctx, items)
	require.Equal(1, txm.Len(ctx), "Item not added.")
	max, ok := txm.PeekMax(ctx)
	require.True(ok)
	require.Equal(uint64(300), max.UnitPrice())
	max, ok = txm.PeekMax(ctx)
	require.True(ok)
	require.Equal(uint64(300), max.UnitPrice())
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
	txm := New[*MempoolTestItem](tracer, 20, 4, [][]byte{exemptPayers})
	// Add 6 transactions for each payer
	for i := uint64(0); i <= 5; i++ {
		itemPayer := GenerateTestItem(payer, 1, i)
		itemExempt := GenerateTestItem(exemptPayer, 1, i)
		items := []*MempoolTestItem{itemPayer, itemExempt}
		txm.Add(ctx, items)
	}
	require.Equal(10, txm.Len(ctx), "Mempool has incorrect txs.")
	require.Equal(4, len(txm.owned[payer]), "Payer has incorrect txs.")
	require.Equal(6, len(txm.owned[exemptPayer]), "Payer has incorrect txs.")
}

func TestMempoolAddExceedMaxSize(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.TODO()
	tracer, _ := trace.New(&trace.Config{Enabled: false})

	txm := New[*MempoolTestItem](tracer, 3, 20, nil)
	// Add more tx's than txm.maxSize
	for i := uint64(0); i <= 9; i++ {
		item := GenerateTestItem(testPayer, 1, i)
		items := []*MempoolTestItem{item}
		txm.Add(ctx, items)
		// Since UnitPrice() is increasing, tx should be included
		require.True(txm.Has(ctx, item.ID()), "TX not included")
	}
	// Pop and check values
	for i := uint64(7); i <= 9; i++ {
		popped, ok := txm.PopMin(ctx)
		require.True(ok)
		require.Equal(i, popped.UnitPrice(), "Mempool did not pop correct tx.")
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

	txm := New[*MempoolTestItem](tracer, 3, 20, nil)
	// Add
	item := GenerateTestItem(testPayer, 1, 10)
	items := []*MempoolTestItem{item}
	txm.Add(ctx, items)
	require.True(txm.Has(ctx, item.ID()), "TX not included")
	// Remove
	itemNotIn := GenerateTestItem(testPayer, 1, 10)
	items = []*MempoolTestItem{item, itemNotIn}
	txm.Remove(ctx, items)
	require.Equal(0, txm.Len(ctx), "Mempool has incorrect number of txs.")
}

func TestMempoolRemoveAccount(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.TODO()
	tracer, _ := trace.New(&trace.Config{Enabled: false})

	txm := New[*MempoolTestItem](tracer, 3, 20, nil)
	// Add
	item1 := GenerateTestItem(testPayer, 1, 10)
	item2 := GenerateTestItem(testPayer, 1, 20)

	items := []*MempoolTestItem{item1, item2}
	txm.Add(ctx, items)
	require.True(txm.Has(ctx, item1.ID()), "TX not included")
	require.True(txm.Has(ctx, item2.ID()), "TX not included")
	txm.RemoveAccount(ctx, testPayer)
	require.Equal(0, txm.Len(ctx), "Mempool has incorrect number of txs.")
	_, owned := txm.owned[testPayer]
	require.False(owned, "Payer not removed from owned.")
}

func TestMempoolSetMinTimestamp(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	ctx := context.TODO()
	tracer, _ := trace.New(&trace.Config{Enabled: false})

	txm := New[*MempoolTestItem](tracer, 20, 20, nil)
	// Add more tx's than txm.maxSize
	for i := int64(0); i <= 9; i++ {
		item := GenerateTestItem(testPayer, i, 10)
		items := []*MempoolTestItem{item}
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

func TestMempoolBuild(t *testing.T) {
	require := require.New(t)
	require.True(true, "not true")
}

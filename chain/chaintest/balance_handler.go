// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

// TestBalanceHandler tests b by requiring that it upholds the invariants
// described in the BalanceHandler interface.
func TestBalanceHandler(t *testing.T, ctx context.Context, bf func() chain.BalanceHandler) {
	addrOne := codectest.NewRandomAddress()
	addrTwo := codectest.NewRandomAddress()

	// AddFeeBalance() tests are not checked for state keys
	t.Run("add balance", func(t *testing.T) {
		r := require.New(t)
		ms := NewInMemoryStore()
		bh := bf()

		r.NoError(bh.AddFeeBalance(ctx, addrOne, ms, 1))

		balance, err := bh.GetFeeBalance(ctx, addrOne, ms)
		r.NoError(err)
		r.Equal(uint64(1), balance)
	})

	t.Run("add balance - overflow", func(t *testing.T) {
		r := require.New(t)
		ms := NewInMemoryStore()
		bh := bf()

		r.NoError(bh.AddFeeBalance(ctx, addrOne, ms, consts.MaxUint64))

		r.Error(bh.AddFeeBalance(ctx, addrOne, ms, 1))

		balance, err := bh.GetFeeBalance(ctx, addrOne, ms)
		r.NoError(err)
		r.Equal(consts.MaxUint64, balance)
	})

	t.Run("add balance - multiple accounts", func(t *testing.T) {
		r := require.New(t)
		ms := NewInMemoryStore()
		bh := bf()

		r.NoError(bh.AddFeeBalance(ctx, addrOne, ms, 1))
		r.NoError(bh.AddFeeBalance(ctx, addrTwo, ms, 2))

		balance, err := bh.GetFeeBalance(ctx, addrOne, ms)
		r.NoError(err)
		r.Equal(uint64(1), balance)

		balance, err = bh.GetFeeBalance(ctx, addrTwo, ms)
		r.NoError(err)
		r.Equal(uint64(2), balance)
	})

	t.Run("deduct", func(t *testing.T) {
		r := require.New(t)
		ms := NewInMemoryStore()
		bh := bf()

		r.NoError(bh.AddFeeBalance(ctx, addrOne, ms, 1))

		ts := tstate.New(1)
		stateKeys := bh.SponsorStateKeys(addrOne)
		tsv := ts.NewView(
			stateKeys,
			state.ImmutableStorage(ms.Storage),
			len(stateKeys),
		)

		r.NoError(bh.DeductFeeBalance(ctx, addrOne, tsv, 1))

		balance, err := bh.GetFeeBalance(ctx, addrOne, tsv)
		r.NoError(err)
		r.Equal(uint64(0), balance)
	})

	t.Run("deduct - not enough balance", func(t *testing.T) {
		r := require.New(t)
		ms := NewInMemoryStore()
		bh := bf()

		r.NoError(bh.AddFeeBalance(ctx, addrOne, ms, 1))

		ts := tstate.New(1)
		stateKeys := bh.SponsorStateKeys(addrOne)
		tsv := ts.NewView(
			stateKeys,
			state.ImmutableStorage(ms.Storage),
			len(stateKeys),
		)

		r.Error(bh.DeductFeeBalance(ctx, addrOne, tsv, 2))

		balance, err := bh.GetFeeBalance(ctx, addrOne, tsv)
		r.NoError(err)
		r.Equal(uint64(1), balance)
	})

	t.Run("can deduct", func(t *testing.T) {
		r := require.New(t)
		ms := NewInMemoryStore()
		bh := bf()

		r.NoError(bh.AddFeeBalance(ctx, addrOne, ms, 1))

		ts := tstate.New(1)
		stateKeys := bh.SponsorStateKeys(addrOne)
		tsv := ts.NewView(
			stateKeys,
			state.ImmutableStorage(ms.Storage),
			len(stateKeys),
		)

		r.NoError(bh.CanDeductFeeBalance(ctx, addrOne, tsv, 1))

		balance, err := bh.GetFeeBalance(ctx, addrOne, tsv)
		r.NoError(err)
		r.Equal(uint64(1), balance)
	})

	t.Run("can deduct - not enough balance", func(t *testing.T) {
		r := require.New(t)
		ms := NewInMemoryStore()
		bh := bf()

		r.NoError(bh.AddFeeBalance(ctx, addrOne, ms, 1))

		ts := tstate.New(1)
		stateKeys := bh.SponsorStateKeys(addrOne)
		tsv := ts.NewView(
			stateKeys,
			state.ImmutableStorage(ms.Storage),
			len(stateKeys),
		)

		r.Error(bh.CanDeductFeeBalance(ctx, addrOne, tsv, 2))

		balance, err := bh.GetFeeBalance(ctx, addrOne, tsv)
		r.NoError(err)
		r.Equal(uint64(1), balance)
	})
}

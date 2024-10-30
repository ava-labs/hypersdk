// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/state/tstate"
)

// TestBalanceHandler tests b by doing the following:
//
// - Creating a new instance of tstate with permissions from SponsorStateKeys().
//   - The test account balance is initialized to 0.
//
// - Call AddBalance() to increment the test account balance by 1.
// - Call GetBalance() and require that the test account balance is 1.
// - Call CanDeduct() to check if the test account balance can be deducted by 1.
// - Call Deduct() to decrement the test account balance by 1.
// - Call GetBalance() and require that the test account balance is 0.
//
// TestBalanceHandler fails if any of the above method calls returns an error.
func TestBalanceHandler(ctx context.Context, b chain.BalanceHandler, t *testing.T) {
	addr := codectest.NewRandomAddress()
	amount := uint64(1)
	store := NewInMemoryStore()

	t.Run("initialize key-value store for addr", func(t *testing.T) {
		r := require.New(t)
		r.NoError(b.AddBalance(ctx, addr, store, uint64(0), true))
	})

	stateKeys := b.SponsorStateKeys(addr)
	ts := tstate.New(1)
	tsv := ts.NewView(
		stateKeys,
		store.Storage,
	)

	t.Run("add balance", func(t *testing.T) {
		r := require.New(t)
		r.NoError(b.AddBalance(ctx, addr, tsv, amount, true))
		balance, err := b.GetBalance(ctx, addr, tsv)
		r.NoError(err)
		r.Equal(amount, balance)
	})

	t.Run("can deduct", func(t *testing.T) {
		r := require.New(t)
		r.NoError(b.CanDeduct(ctx, addr, tsv, amount))
	})

	t.Run("deduct balance", func(t *testing.T) {
		r := require.New(t)
		r.NoError(b.Deduct(ctx, addr, tsv, amount))
		balance, err := b.GetBalance(ctx, addr, tsv)
		r.NoError(err)
		r.Equal(uint64(0), balance)
	})
}

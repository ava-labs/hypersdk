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

// TestBalanceHandler tests [b] by calling the following methods (in order):
// - SponsorStateKeys()
// - AddBalance()
// - CanDeduct()
func TestBalanceHandler(t *testing.T, b chain.BalanceHandler) {
	r := require.New(t)
	addr := codectest.NewRandomAddress()
	amount := uint64(1)

	// Initialize key-value store for addr
	store := NewInMemoryStore()
	r.NoError(b.AddBalance(context.Background(), addr, store, amount, true))

	stateKeys := b.SponsorStateKeys(addr)
	ts := tstate.New(1)
	tsv := ts.NewView(
		stateKeys,
		store.Storage,
	)

	r.NoError(b.AddBalance(context.Background(), addr, tsv, 0, true))
	balance, err := b.GetBalance(context.Background(), addr, tsv)
	r.NoError(err)
	r.Equal(amount, balance)

	r.NoError(b.CanDeduct(context.Background(), addr, tsv, amount))

	r.NoError(b.Deduct(context.Background(), addr, tsv, amount))
	balance, err = b.GetBalance(context.Background(), addr, tsv)
	r.NoError(err)
	r.Equal(uint64(0), balance)
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

// TestBalanceHandler tests b by requiring that it upholds the invariants
// described in the BalanceHandler interface.
func TestBalanceHandler(ctx context.Context, t *testing.T, b chain.BalanceHandler) {
	// Test addresses
	addrOne := codectest.NewRandomAddress()
	addrTwo := codectest.NewRandomAddress()

	tests := []struct {
		Name string

		Actor  codec.Address
		Amount uint64

		// State keys are not enforced in Setup
		Setup func(context.Context, *testing.T, state.Mutable)
		// State keys are enforced in TestFunc
		TestFunc func(context.Context, codec.Address, state.Mutable, uint64) error
		// Assertion is run after TestFunc for any additional checks
		// State keys are not enforced in Assertion
		Assertion func(context.Context, *testing.T, state.Mutable)

		ExpectedBalance uint64
		ExpectErr       bool
	}{
		{
			Name:   "simpleAddBalance",
			Actor:  addrOne,
			Amount: 1,
			Setup: func(ctx context.Context, t *testing.T, mu state.Mutable) {
				r := require.New(t)
				r.NoError(b.AddBalance(ctx, addrOne, mu, 0))
			},
			TestFunc:        b.AddBalance,
			ExpectedBalance: 1,
		},
		{
			Name:   "addBalanceFailsIfOverflow",
			Actor:  addrOne,
			Amount: 1,
			Setup: func(ctx context.Context, t *testing.T, mu state.Mutable) {
				// Initialize addrOne balance to MaxUint64
				r := require.New(t)
				r.NoError(b.AddBalance(ctx, addrOne, mu, consts.MaxUint64))
			},
			TestFunc:        b.AddBalance,
			ExpectedBalance: consts.MaxUint64,
			ExpectErr:       true,
		},
		{
			Name:   "simpleDeduct",
			Actor:  addrOne,
			Amount: 1,
			Setup: func(ctx context.Context, t *testing.T, mu state.Mutable) {
				// Initialize addrOne balance to 1
				r := require.New(t)
				r.NoError(b.AddBalance(ctx, addrOne, mu, 1))
			},
			TestFunc:        b.Deduct,
			ExpectedBalance: 0,
		},
		{
			Name:   "deductFailsIfNotEnoughBalance",
			Actor:  addrOne,
			Amount: 2,
			Setup: func(ctx context.Context, t *testing.T, mu state.Mutable) {
				// Initialize addrOne balance to 1
				r := require.New(t)
				r.NoError(b.AddBalance(ctx, addrOne, mu, 1))
			},
			TestFunc:        b.Deduct,
			ExpectedBalance: 1,
			ExpectErr:       true,
		},
		{
			Name:   "simpleCanDeduct",
			Actor:  addrOne,
			Amount: 1,
			Setup: func(
				ctx context.Context,
				t *testing.T,
				mu state.Mutable,
			) {
				// Initialize addrOne balance to 1
				r := require.New(t)
				r.NoError(b.AddBalance(ctx, addrOne, mu, 1))
			},
			TestFunc: func(
				ctx context.Context,
				addr codec.Address,
				mu state.Mutable,
				amount uint64,
			) error {
				return b.CanDeduct(ctx, addr, mu, amount)
			},
			ExpectedBalance: 1,
		},
		{
			Name:   "canDeductFailsIfNotEnoughBalance",
			Actor:  addrOne,
			Amount: 2,
			Setup: func(ctx context.Context, t *testing.T, mu state.Mutable) {
				// Initialize addrOne balance to 1
				r := require.New(t)
				r.NoError(b.AddBalance(ctx, addrOne, mu, 1))
			},
			TestFunc: func(
				ctx context.Context,
				addr codec.Address,
				mu state.Mutable,
				amount uint64,
			) error {
				return b.CanDeduct(ctx, addr, mu, amount)
			},
			ExpectedBalance: 1,
			ExpectErr:       true,
		},
		{
			Name:   "multipleAccountsCanExist",
			Actor:  addrTwo,
			Amount: 2,
			Setup: func(ctx context.Context, t *testing.T, mu state.Mutable) {
				r := require.New(t)
				// Initialize addrOne balance to 1
				r.NoError(b.AddBalance(ctx, addrOne, mu, 1))
				// Initialize addrTwo balance to 0
				r.NoError(b.AddBalance(ctx, addrTwo, mu, 0))
			},
			TestFunc:        b.AddBalance,
			ExpectedBalance: 2,
			Assertion: func(ctx context.Context, t *testing.T, mu state.Mutable) {
				// Check that addrOne balance is still 1
				r := require.New(t)
				balance, err := b.GetBalance(ctx, addrOne, mu)
				r.NoError(err)
				r.Equal(uint64(1), balance)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Set up test state
			r := require.New(t)

			ms := NewInMemoryStore()
			if tt.Setup != nil {
				tt.Setup(ctx, t, ms)
			}

			ts := tstate.New(1)
			tsv := ts.NewView(
				b.SponsorStateKeys(tt.Actor),
				ms.Storage,
			)

			// Run test
			if tt.ExpectErr {
				r.Error(tt.TestFunc(ctx, tt.Actor, tsv, tt.Amount))
			} else {
				r.NoError(tt.TestFunc(ctx, tt.Actor, tsv, tt.Amount))
			}

			balance, err := b.GetBalance(ctx, tt.Actor, tsv)
			r.NoError(err)
			r.Equal(tt.ExpectedBalance, balance)

			if tt.Assertion != nil {
				tt.Assertion(ctx, t, ms)
			}
		})
	}
}

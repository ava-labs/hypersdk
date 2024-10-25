// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

const (
	TestCanDeduct BalanceHandlerMethod = iota
	TestDeduct
	TestAddBalance
)

type BalanceHandlerMethod uint8

// BalanceHandlerStateKeyTest is a single parameterized test. It calls
// AddBalance(),
// CanDeduct(), and
// Deduct() on [BalanceHandler] in that order and checks that no errors are returned
type BalanceHandlerStateKeyTest struct {
	Name string

	BalanceHandler chain.BalanceHandler

	Actor  codec.Address
	Amount uint64
}

// Run executes the [BalanceHandlerStateKeyTest] test and makes sure that no
// errors are returned
//
// Run creates a new state with state keys from SponsorStateKeys()
func (test *BalanceHandlerStateKeyTest) Run(ctx context.Context, t *testing.T) {
	t.Run(test.Name, func(t *testing.T) {
		r := require.New(t)

		ms := NewInMemoryStore()

		// Add balance
		r.NoError(test.BalanceHandler.AddBalance(ctx, test.Actor, ms, test.Amount, true))

		stateKeys := test.BalanceHandler.SponsorStateKeys(test.Actor)
		ts := tstate.New(1)
		tsv := ts.NewView(stateKeys, ms.Storage)

		// Add balance
		r.NoError(test.BalanceHandler.AddBalance(ctx, test.Actor, tsv, test.Amount, true))
		// Check if can deduct
		r.NoError(test.BalanceHandler.CanDeduct(ctx, test.Actor, tsv, test.Amount))
		// Deduct balance
		r.NoError(test.BalanceHandler.Deduct(ctx, test.Actor, tsv, test.Amount))
	})
}

// BalanceHandlerFundsTest is a single parameterized test. It calls the
// specified method with the passed parameters and checks that all assertions pass.
type BalanceHandlerFundsTest struct {
	Name string

	BalanceHandler chain.BalanceHandler

	Actor         codec.Address
	Amount        uint64
	State         state.Mutable
	CreateAccount bool

	Method          BalanceHandlerMethod
	ExpectedBalance uint64
	ExpectedError   error

	Assertion func(context.Context, *testing.T, state.Mutable)
}

// Run executes the [BalanceHandlerFundsTest] and makes sure all assertions pass.
func (test *BalanceHandlerFundsTest) Run(ctx context.Context, t *testing.T) {
	t.Run(test.Name, func(t *testing.T) {
		r := require.New(t)

		switch test.Method {
		case TestCanDeduct:
			r.ErrorIs(test.BalanceHandler.CanDeduct(ctx, test.Actor, test.State, test.Amount), test.ExpectedError)
		case TestDeduct:
			r.ErrorIs(test.BalanceHandler.Deduct(ctx, test.Actor, test.State, test.Amount), test.ExpectedError)
		case TestAddBalance:
			r.ErrorIs(test.BalanceHandler.AddBalance(ctx, test.Actor, test.State, test.Amount, test.CreateAccount), test.ExpectedError)
		default:
			r.Fail("no test method specified")
		}

		balance, err := test.BalanceHandler.GetBalance(ctx, test.Actor, test.State)
		r.NoError(err)

		r.Equal(test.ExpectedBalance, balance)

		if test.Assertion != nil {
			test.Assertion(ctx, t, test.State)
		}
	})
}

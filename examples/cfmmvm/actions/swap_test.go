// Copyright (C) 2024, Ava Labs, Inc. All rights reservea.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/pricing"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

func TestSwap(t *testing.T) {
	req := require.New(t)
	ts := tstate.New(1)

	addr := codectest.NewRandomAddress()

	parentState := ts.NewView(
		state.Keys{
			string(storage.TokenInfoKey(tokenOneAddress)):                              state.All,
			string(storage.TokenInfoKey(tokenTwoAddress)):                              state.All,
			string(storage.LiquidityPoolKey(lpAddress)):                                state.All,
			string(storage.TokenInfoKey(lpTokenAddress)):                               state.All,
			string(storage.TokenAccountBalanceKey(tokenOneAddress, addr)):              state.All,
			string(storage.TokenAccountBalanceKey(tokenTwoAddress, addr)):              state.All,
			string(storage.TokenAccountBalanceKey(tokenOneAddress, lpAddress)):         state.All,
			string(storage.TokenAccountBalanceKey(tokenTwoAddress, lpAddress)):         state.All,
			string(storage.TokenAccountBalanceKey(lpTokenAddress, codec.EmptyAddress)): state.All,
			string(storage.TokenAccountBalanceKey(lpTokenAddress, addr)):               state.All,
		},
		chaintest.NewInMemoryStore().Storage,
	)

	setupActions := []chain.Action{
		&CreateToken{
			Name:     []byte(TokenOneName),
			Symbol:   []byte(TokenOneSymbol),
			Metadata: []byte(TokenOneMetadata),
		},
		&CreateToken{
			Name:     []byte(TokenTwoName),
			Symbol:   []byte(TokenTwoSymbol),
			Metadata: []byte(TokenTwoMetadata),
		},
		&MintToken{
			To:    addr,
			Value: 10_000 + InitialSwapValue,
			Token: tokenOneAddress,
		},
		&MintToken{
			To:    addr,
			Value: 10_000,
			Token: tokenTwoAddress,
		},
	}

	for _, action := range setupActions {
		_, err := action.Execute(context.Background(), nil, parentState, 0, addr, ids.Empty)
		req.NoError(err)
	}

	test := chaintest.ActionTest{
		Name: "LP must exist",
		Action: &Swap{
			LPAddress: lpAddress,
		},
		ExpectedOutputs: nil,
		ExpectedErr:     ErrOutputLiquidityPoolDoesNotExist,
		State:           parentState,
	}

	test.Run(context.Background(), t)

	setupActions = []chain.Action{
		&CreateLiquidityPool{
			FunctionID: pricing.ConstantProductID,
			TokenX:     tokenOneAddress,
			TokenY:     tokenTwoAddress,
			Fee:        InitialFee,
		},
		&AddLiquidity{
			AmountX:       10_000,
			AmountY:       10_000,
			LiquidityPool: lpAddress,
		},
	}

	for _, action := range setupActions {
		_, err := action.Execute(context.Background(), nil, parentState, 0, addr, ids.Empty)
		req.NoError(err)
	}

	tests := []chaintest.ActionTest{
		{
			Name: "Both deltas cannot be zero",
			Action: &Swap{
				AmountXIn: 0,
				AmountYIn: 0,
				LPAddress: lpAddress,
			},
			ExpectedOutputs: nil,
			ExpectedErr:     pricing.ErrBothDeltasZero,
			State:           parentState,
			Actor:           addr,
		},
		{
			Name: "Both deltas cannot be nonzero",
			Action: &Swap{
				AmountXIn: 1,
				AmountYIn: 1,
				LPAddress: lpAddress,
			},
			ExpectedOutputs: nil,
			ExpectedErr:     pricing.ErrNoClearDeltaToCompute,
			State:           parentState,
			Actor:           addr,
		},
		{
			Name: "Correct swap should work",
			Action: &Swap{
				AmountXIn: InitialSwapValue,
				AmountYIn: 0,
				LPAddress: lpAddress,
			},
			ExpectedOutputs: &SwapResult{},
			ExpectedErr:     nil,
			State:           parentState,
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				require := require.New(t)
				balance, err := storage.GetTokenAccountBalanceNoController(ctx, m, tokenTwoAddress, addr)
				require.NoError(err)
				require.Equal(uint64(99), balance)
			},
			Actor: addr,
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

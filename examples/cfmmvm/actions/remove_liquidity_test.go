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

func TestRemoveLiquidity(t *testing.T) {
	req := require.New(t)
	ts = tstate.New(1)

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

	test := chaintest.ActionTest{
		Name: "LP must exist",
		Action: &RemoveLiquidity{
			LiquidityPool: lpAddress,
		},
		ExpectedOutputs: nil,
		ExpectedErr:     ErrOutputLiquidityPoolDoesNotExist,
		State:           parentState,
	}

	test.Run(context.Background(), t)

	setupActions := []chain.Action{
		&CreateToken{
			[]byte(TokenOneName),
			[]byte(TokenOneSymbol),
			[]byte(TokenOneMetadata),
		},
		&CreateToken{
			[]byte(TokenTwoName),
			[]byte(TokenTwoSymbol),
			[]byte(TokenTwoMetadata),
		},
		&CreateLiquidityPool{
			pricing.ConstantProductID,
			tokenOneAddress,
			tokenTwoAddress,
			InitialFee,
		},
		&MintToken{
			To:    addr,
			Value: 10_000,
			Token: tokenOneAddress,
		},
		&MintToken{
			To:    addr,
			Value: 10_000,
			Token: tokenTwoAddress,
		},
		&AddLiquidity{
			LiquidityPool: lpAddress,
			AmountX:       10_000,
			AmountY:       10_000,
		},
	}

	for _, action := range setupActions {
		_, err := action.Execute(context.Background(), nil, parentState, 0, addr, ids.Empty)
		req.NoError(err)
	}

	tests := []chaintest.ActionTest{
		{
			Name: "Correct instance of removing liquidity",
			Action: &RemoveLiquidity{
				BurnAmount:    1_000,
				LiquidityPool: lpAddress,
				TokenX:        tokenOneAddress,
				TokenY:        tokenTwoAddress,
			},
			ExpectedOutputs: &RemoveLiquidityResult{
				TokenXAmount: 1_000,
				TokenYAmount: 1_000,
			},
			ExpectedErr:     nil,
			State:           parentState,
			Actor:           addr,
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				require := require.New(t)
				tokenOneBalance, err := storage.GetTokenAccountBalanceNoController(context.TODO(), m, tokenOneAddress, addr)
				require.NoError(err)
				tokenTwoBalance, err := storage.GetTokenAccountBalanceNoController(context.TODO(), m, tokenTwoAddress, addr)
				require.NoError(err)
				require.Equal(uint64(1_000), tokenOneBalance)
				require.Equal(uint64(1_000), tokenTwoBalance)
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

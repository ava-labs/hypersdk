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

	smath "github.com/ava-labs/avalanchego/utils/math"
	hconsts "github.com/ava-labs/hypersdk/consts"
)

func TestAddLiquidity(t *testing.T) {
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
		Action: &AddLiquidity{
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
			Value: hconsts.MaxUint64,
			Token: tokenOneAddress,
		},
		&MintToken{
			To:    addr,
			Value: hconsts.MaxUint64,
			Token: tokenTwoAddress,
		},
	}

	for _, action := range setupActions {
		_, err := action.Execute(context.Background(), nil, parentState, 0, addr, ids.Empty)
		req.NoError(err)
	}

	tests := []chaintest.ActionTest{
		{
			Name: "Initial liquidity minted must be >= Minimum Liquidity",
			Action: &AddLiquidity{
				AmountX:       1,
				AmountY:       1,
				LiquidityPool: lpAddress,
			},
			ExpectedOutputs: nil,
			ExpectedErr:     smath.ErrUnderflow,
			State:           parentState,
			Actor:           addr,
		},
		{
			Name: "Liquidity integer overflow (No Liquidity)",
			Action: &AddLiquidity{
				AmountX:       hconsts.MaxUint64,
				AmountY:       hconsts.MaxUint64,
				LiquidityPool: lpAddress,
			},
			ExpectedOutputs: nil,
			ExpectedErr:     smath.ErrOverflow,
			State:           parentState,
			Actor:           addr,
		},
		{
			Name: "Basic liquidity deposit works",
			Action: &AddLiquidity{
				AmountX:       10_000,
				AmountY:       10_000,
				LiquidityPool: lpAddress,
			},
			ExpectedErr:     nil,
			ExpectedOutputs: &AddLiquidityResult{},
			State:           parentState,
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				require := require.New(t)
				// Check LP-state
				functionID, tokenX, tokenY, fee, feeTo, reserveX, reserveY, lpTokenAddressFromChain, kLast, err := storage.GetLiquidityPoolNoController(context.TODO(), m, lpAddress)
				require.NoError(err)
				_, _, _, tSupply, _, err := storage.GetTokenInfoNoController(context.Background(), m, lpTokenAddressFromChain)
				require.NoError(err)
				zeroAddressBalance, err := storage.GetTokenAccountBalanceNoController(context.TODO(), m, lpTokenAddressFromChain, codec.EmptyAddress)
				require.NoError(err)
				addrBalance, err := storage.GetTokenAccountBalanceNoController(context.TODO(), m, lpTokenAddressFromChain, addr)
				require.NoError(err)
				require.Equal(pricing.ConstantProductID, functionID)
				require.Equal(tokenOneAddress, tokenX)
				require.Equal(tokenTwoAddress, tokenY)
				require.Equal(InitialFee, fee)
				require.Equal(addr, feeTo)
				require.Equal(uint64(10_000), reserveX)
				require.Equal(uint64(10_000), reserveY)
				require.Equal(lpTokenAddress, lpTokenAddressFromChain)
				require.Equal(uint64(10_000*10_000), kLast)
				require.Equal(uint64(10_000), tSupply)
				require.Equal(uint64(9_000), addrBalance)
				require.Equal(uint64(1_000), zeroAddressBalance)
				// Check LP balances of tokens
				tokenOneBalance, err := storage.GetTokenAccountBalanceNoController(ctx, m, tokenOneAddress, lpAddress)
				require.NoError(err)
				tokenTwoBalance, err := storage.GetTokenAccountBalanceNoController(ctx, m, tokenTwoAddress, lpAddress)
				require.NoError(err)
				require.Equal(uint64(10_000), tokenOneBalance)
				require.Equal(uint64(10_000), tokenTwoBalance)
			},
			Actor: addr,
		},
		{
			Name: "Liquidity must not equal 0",
			Action: &AddLiquidity{
				LiquidityPool: lpAddress,
			},
			ExpectedOutputs: nil,
			ExpectedErr:     pricing.ErrOutputInsufficientLiquidityMinted,
			State:           parentState,
			Actor:           addr,
		},
		{
			Name: "Correct instance of supplying liquidity to a pool with existing liquidity",
			Action: &AddLiquidity{
				AmountX:       5_000,
				AmountY:       5_000,
				LiquidityPool: lpAddress,
			},
			ExpectedOutputs: &AddLiquidityResult{},
			ExpectedErr:     nil,
			State:           parentState,
			Actor:           addr,
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				require := require.New(t)
				// Check LP provider state
				addrLPTokenBalance, err := storage.GetTokenAccountBalanceNoController(context.TODO(), m, lpTokenAddress, addr)
				require.NoError(err)
				require.Equal(uint64(14_000), addrLPTokenBalance)
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

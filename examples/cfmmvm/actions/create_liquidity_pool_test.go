// Copyright (C) 2024, Ava Labs, Inc. All rights reservea.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/pricing"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"
	"github.com/ava-labs/hypersdk/internal/state/tstate"
	"github.com/ava-labs/hypersdk/state"
)

func TestCreateLiquidityPool(t *testing.T) {
	req := require.New(t)
	ts := tstate.New(1)

	addr, err := codectest.NewRandomAddress()
	req.NoError(err)

	parentState := ts.NewView(
		state.Keys{
			string(storage.TokenInfoKey(tokenOneAddress)): state.All,
			string(storage.TokenInfoKey(tokenTwoAddress)): state.All,
			string(storage.LiquidityPoolKey(lpAddress)):   state.All,
			string(storage.TokenInfoKey(lpTokenAddress)):  state.All,
		},
		chaintest.NewInMemoryStore().Storage,
	)

	tests := []chaintest.ActionTest{
		{
			Name: "Invalid fee not allowed",
			Action: &CreateLiquidityPool{
				FunctionID: pricing.ConstantProductID,
				TokenX:     tokenOneAddress,
				TokenY:     tokenTwoAddress,
				Fee:        0,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputInvalidFee,
			State:           parentState,
		},
		{
			Name: "Token X must exist",
			Action: &CreateLiquidityPool{
				FunctionID: pricing.ConstantProductID,
				TokenX:     codec.EmptyAddress,
				TokenY:     tokenTwoAddress,
				Fee:        InitialFee,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenXDoesNotExist,
			State:           parentState,
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}

	req.NoError(storage.SetTokenInfo(context.Background(), parentState, tokenOneAddress, []byte(TokenOneName), []byte(TokenOneSymbol), []byte(TokenOneMetadata), 0, addr))

	tests = []chaintest.ActionTest{
		{
			Name: "Token Y must exist",
			Action: &CreateLiquidityPool{
				FunctionID: pricing.ConstantProductID,
				TokenX:     tokenOneAddress,
				TokenY:     codec.EmptyAddress,
				Fee:        InitialFee,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenYDoesNotExist,
			State:           parentState,
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}

	req.NoError(storage.SetTokenInfo(context.Background(), parentState, tokenTwoAddress, []byte(TokenTwoName), []byte(TokenTwoSymbol), []byte(TokenTwoMetadata), 0, addr))

	tests = []chaintest.ActionTest{
		{
			Name: "No invalid model IDs",
			Action: &CreateLiquidityPool{
				FunctionID: pricing.InvalidModelID,
				TokenX:     tokenOneAddress,
				TokenY:     tokenTwoAddress,
				Fee:        InitialFee,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputFunctionDoesNotExist,
			State:           parentState,
		},
		{
			Name: "Correct liquidity pool creations are allowed",
			Action: &CreateLiquidityPool{
				FunctionID: pricing.ConstantProductID,
				TokenX:     tokenOneAddress,
				TokenY:     tokenTwoAddress,
				Fee:        InitialFee,
			},
			ExpectedErr:     nil,
			ExpectedOutputs: [][]byte{lpAddress[:], lpTokenAddress[:]},
			State:           parentState,
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				require := require.New(t)
				// Assert correct lp instantiation
				functionID, tokenX, tokenY, fee, feeTo, reserveX, reserveY, lpTokenAddressFromChain, kLast, err := storage.GetLiquidityPoolNoController(context.TODO(), m, lpAddress)
				require.NoError(err)
				require.Equal(pricing.ConstantProductID, functionID)
				require.Equal(tokenOneAddress, tokenX)
				require.Equal(tokenTwoAddress, tokenY)
				require.Equal(InitialFee, fee)
				require.Equal(addr, feeTo)
				require.Equal(uint64(0), reserveX)
				require.Equal(uint64(0), reserveY)
				require.Equal(lpTokenAddress, lpTokenAddressFromChain)
				require.Equal(uint64(0), kLast)
				// Assert correct lp token instantiation
				name, symbol, metadata, totalSupply, owner, err := storage.GetTokenInfoNoController(ctx, m, lpTokenAddress)
				require.NoError(err)
				require.Equal(storage.LiquidityPoolTokenName, string(name))
				require.Equal(storage.LiquidityPoolTokenSymbol, string(symbol))
				require.Equal(storage.LiquidityPoolTokenMetadata, string(metadata))
				require.Equal(uint64(0), totalSupply)
				require.Equal(lpAddress, owner)
			},
			Actor: addr,
		},
		{
			Name: "No overwriting existing LP pool",
			Action: &CreateLiquidityPool{
				FunctionID: pricing.ConstantProductID,
				TokenX:     tokenOneAddress,
				TokenY:     tokenTwoAddress,
				Fee:        InitialFee,
			},
			ExpectedErr:     ErrOutputLiquidityPoolAlreadyExists,
			ExpectedOutputs: [][]byte(nil),
			State:           parentState,
		},
	}
	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

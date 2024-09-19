// Copyright (C) 2024, Ava Labs, Inc. All rights reservea.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

func TestBurnToken(t *testing.T) {
	req := require.New(t)
	ts := tstate.New(1)

	addr := codectest.NewRandomAddress()

	addrTwo := codectest.NewRandomAddress()

	parentState := ts.NewView(
		state.Keys{
			string(storage.TokenInfoKey(tokenOneAddress)):                    state.All,
			string(storage.TokenAccountBalanceKey(tokenOneAddress, addr)):    state.All,
			string(storage.TokenAccountBalanceKey(tokenOneAddress, addrTwo)): state.All,
		},
		chaintest.NewInMemoryStore().Storage,
	)

	tests := []chaintest.ActionTest{
		{
			Name: "Burn value must be greater than 0",
			Action: &BurnToken{
				TokenAddress: tokenOneAddress,
				Value:        0,
			},
			State:           parentState,
			ExpectedOutputs: nil,
			ExpectedErr:     ErrOutputBurnValueZero,
		},
		{
			Name: "Can only burn existing tokens",
			Action: &BurnToken{
				TokenAddress: tokenOneAddress,
				Value:        InitialTokenBurnValue,
			},
			State:           parentState,
			ExpectedOutputs: nil,
			ExpectedErr:     ErrOutputTokenDoesNotExist,
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}

	req.NoError(storage.SetTokenInfo(context.Background(), parentState, tokenOneAddress, []byte(TokenOneName), []byte(TokenOneSymbol), []byte(TokenOneMetadata), InitialTokenMintValue, addr))
	req.NoError(storage.SetTokenAccountBalance(context.Background(), parentState, tokenOneAddress, addr, InitialTokenMintValue))

	tests = []chaintest.ActionTest{
		{
			Name: "Burn value must be greater or equal to than actor balance",
			Action: &BurnToken{
				TokenAddress: tokenOneAddress,
				Value:        InitialTokenBurnValue,
			},
			State:           parentState,
			ExpectedOutputs: nil,
			ExpectedErr:     ErrOutputInsufficientTokenBalance,
			Actor:           addrTwo,
		},
		{
			Name: "Correct burns can occur",
			Action: &BurnToken{
				TokenAddress: tokenOneAddress,
				Value:        InitialTokenBurnValue,
			},
			State:           parentState,
			ExpectedOutputs: &BurnTokenResult{},
			ExpectedErr:     nil,
			Actor:           addr,
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				require := require.New(t)
				_, _, _, totalSupply, _, err := storage.GetTokenInfoNoController(ctx, m, tokenOneAddress)
				require.NoError(err)
				require.Equal(uint64(0), totalSupply)
				balance, err := storage.GetTokenAccountBalanceNoController(ctx, m, tokenOneAddress, addr)
				require.NoError(err)
				require.Equal(uint64(0), balance)
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

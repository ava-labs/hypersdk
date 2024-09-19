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

func TestMintToken(t *testing.T) {
	req := require.New(t)
	ts := tstate.New(1)

	addr := codectest.NewRandomAddress()

	addrTwo := codectest.NewRandomAddress()

	parentState := ts.NewView(
		state.Keys{
			string(storage.TokenInfoKey(tokenOneAddress)):                 state.All,
			string(storage.TokenAccountBalanceKey(tokenOneAddress, addr)): state.All,
		},
		chaintest.NewInMemoryStore().Storage,
	)

	tests := []chaintest.ActionTest{
		{
			Name: "Mint value must be positive",
			Action: &MintToken{
				To:    addr,
				Token: tokenOneAddress,
				Value: 0,
			},
			ExpectedOutputs: nil,
			ExpectedErr:     ErrOutputMintValueZero,
			State:           parentState,
		},
		{
			Name: "Can only mint existing tokens",
			Action: &MintToken{
				To:    addr,
				Token: tokenOneAddress,
				Value: InitialTokenMintValue,
			},
			ExpectedOutputs: nil,
			ExpectedErr:     ErrOutputTokenDoesNotExist,
			State:           parentState,
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}

	req.NoError(storage.SetTokenInfo(context.Background(), parentState, tokenOneAddress, []byte(TokenOneName), []byte(TokenOneSymbol), []byte(TokenOneMetadata), 0, addr))

	tests = []chaintest.ActionTest{
		{
			Name: "Correct mints can occur",
			Action: &MintToken{
				To:    addr,
				Token: tokenOneAddress,
				Value: InitialTokenMintValue,
			},
			ExpectedOutputs: &MintTokenResult{},
			ExpectedErr:     nil,
			State:           parentState,
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				require := require.New(t)
				_, _, _, totalSupply, _, err := storage.GetTokenInfoNoController(ctx, m, tokenOneAddress)
				require.NoError(err)
				require.Equal(InitialTokenMintValue, totalSupply)
				balance, err := storage.GetTokenAccountBalanceNoController(ctx, m, tokenOneAddress, addr)
				require.NoError(err)
				require.Equal(InitialTokenMintValue, balance)
			},
			Actor: addr,
		},
		{
			Name: "Only owner can mint",
			Action: &MintToken{
				To:    addr,
				Token: tokenOneAddress,
				Value: InitialTokenMintValue,
			},
			ExpectedOutputs: nil,
			ExpectedErr:     ErrOutputTokenNotOwner,
			State:           parentState,
			Actor:           addrTwo,
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

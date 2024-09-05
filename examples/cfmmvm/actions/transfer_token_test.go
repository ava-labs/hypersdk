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
	"github.com/ava-labs/hypersdk/internal/state/tstate"
	"github.com/ava-labs/hypersdk/state"
)

func TestTransferToken(t *testing.T) {
	req := require.New(t)
	ts := tstate.New(1)

	addr, err := codectest.NewRandomAddress()
	req.NoError(err)

	addrTwo, err := codectest.NewRandomAddress()
	req.NoError(err)

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
			Name: "Can only transfer existing tokens",
			Action: &TransferToken{
				To:           addr,
				TokenAddress: tokenOneAddress,
				Value:        TokenTransferValue,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenDoesNotExist,
			State:           parentState,
		},
		{
			Name: "Tranfer value must be greater than 0",
			Action: &TransferToken{
				To:           addr,
				TokenAddress: tokenOneAddress,
				Value:        0,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTransferValueZero,
			State:           parentState,
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}

	req.NoError(storage.SetTokenInfo(context.Background(), parentState, tokenOneAddress, []byte(TokenOneName), []byte(TokenOneSymbol), []byte(TokenOneMetadata), InitialTokenMintValue, addr))
	req.NoError(storage.SetTokenAccountBalance(context.Background(), parentState, tokenOneAddress, addr, InitialTokenMintValue))

	tests = []chaintest.ActionTest{
		{
			Name: "Transfer value must be greater or equal to than sender balance",
			Action: &TransferToken{
				To:           addr,
				TokenAddress: tokenOneAddress,
				Value:        TokenTransferValue,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputInsufficientTokenBalance,
			State:           parentState,
			Actor:           addrTwo,
		},
		{
			Name: "Correct transfers can occur",
			Action: &TransferToken{
				To:           addrTwo,
				TokenAddress: tokenOneAddress,
				Value:        TokenTransferValue,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     nil,
			State:           parentState,
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				require := require.New(t)
				addrBalance, err := storage.GetTokenAccountBalanceNoController(ctx, m, tokenOneAddress, addr)
				require.NoError(err)
				require.Equal(uint64(0), addrBalance)
				addrTwoBalance, err := storage.GetTokenAccountBalanceNoController(ctx, m, tokenOneAddress, addrTwo)
				require.NoError(err)
				require.Equal(TokenTransferValue, addrTwoBalance)
				_, _, _, totalSupply, _, err := storage.GetTokenInfoNoController(ctx, m, tokenOneAddress)
				require.NoError(err)
				require.Equal(InitialTokenMintValue, totalSupply)
			},
			Actor: addr,
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

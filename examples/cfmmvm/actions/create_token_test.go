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

func TestCreateToken(t *testing.T) {
	req := require.New(t)
	ts := tstate.New(1)

	addr, err := codectest.NewRandomAddress()
	req.NoError(err)

	parentState := ts.NewView(
		state.Keys{
			string(storage.TokenInfoKey(tokenOneAddress)): state.All,
		},
		chaintest.NewInMemoryStore().Storage,
	)

	tests := []chaintest.ActionTest{
		{
			Name: "No token with empty name",
			Action: &CreateToken{
				Name:     []byte{},
				Symbol:   []byte(TokenOneSymbol),
				Metadata: []byte(TokenOneMetadata),
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenNameEmpty,
			State:           parentState,
		},
		{
			Name: "No token with empty symbol",
			Action: &CreateToken{
				Name:     []byte(TokenOneName),
				Symbol:   []byte{},
				Metadata: []byte(TokenOneMetadata),
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenSymbolEmpty,
			State:           parentState,
		},
		{
			Name: "No token with empty metadata",
			Action: &CreateToken{
				Name:     []byte(TokenOneName),
				Symbol:   []byte(TokenOneSymbol),
				Metadata: []byte{},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenMetadataEmpty,
			State:           parentState,
		},
		{
			Name: "No token with too large name",
			Action: &CreateToken{
				Name:     []byte(TooLargeTokenName),
				Symbol:   []byte(TokenOneSymbol),
				Metadata: []byte(TokenOneMetadata),
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenNameTooLarge,
			State:           parentState,
		},
		{
			Name: "No token with too large symbol",
			Action: &CreateToken{
				Name:     []byte(TokenOneName),
				Symbol:   []byte(TooLargeTokenSymbol),
				Metadata: []byte(TokenOneMetadata),
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenSymbolTooLarge,
			State:           parentState,
		},
		{
			Name: "No token with too large metadata",
			Action: &CreateToken{
				Name:     []byte(TokenOneName),
				Symbol:   []byte(TokenOneSymbol),
				Metadata: []byte(TooLargeTokenMetadata),
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenMetadataTooLarge,
			State:           parentState,
		},
		{
			Name: "Correct token creation is allowed",
			Action: &CreateToken{
				Name:     []byte(TokenOneName),
				Symbol:   []byte(TokenOneSymbol),
				Metadata: []byte(TokenOneMetadata),
			},
			ExpectedOutputs: [][]byte{tokenOneAddress[:]},
			ExpectedErr:     nil,
			State:           parentState,
			Assertion: func(ctx context.Context, t *testing.T, m state.Mutable) {
				require := require.New(t)
				name, symbol, metadata, totalSupply, owner, err := storage.GetTokenInfoNoController(ctx, m, tokenOneAddress)
				require.NoError(err)
				require.Equal(TokenOneName, string(name))
				require.Equal(TokenOneSymbol, string(symbol))
				require.Equal(TokenOneMetadata, string(metadata))
				require.Equal(uint64(0), totalSupply)
				require.Equal(addr, owner)
			},
			Actor: addr,
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}

	req.NoError(storage.SetTokenInfo(context.Background(), parentState, tokenOneAddress, []byte(TokenOneName), []byte(TokenOneSymbol), []byte(TokenOneMetadata), 0, addr))

	tests = []chaintest.ActionTest{
		{
			Name: "No overwriting existing tokens",
			Action: &CreateToken{
				Name:     []byte(TokenOneName),
				Symbol:   []byte(TokenOneSymbol),
				Metadata: []byte(TokenOneMetadata),
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenAlreadyExists,
			State:           parentState,
			Actor:           addr,
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

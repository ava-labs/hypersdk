// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/chaintest"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
)

func TestCreateToken(t *testing.T) {
	require := require.New(t)
	ts := tstate.New(1)

	onesAddr, err := createAddressWithSameDigits(1)
	require.NoError(err)

	createTokenTests := []chaintest.ActionTest{
		{
			Name: "No token with empty name",
			Action: &CreateToken{
				Name:     []byte{},
				Symbol:   []byte(TokenOneSymbol),
				Metadata: []byte(TokenOneMetadata),
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenNameEmpty,
			State:           GenerateEmptyState(),
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
			State:           GenerateEmptyState(),
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
			State:           GenerateEmptyState(),
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
			State:           GenerateEmptyState(),
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
			State:           GenerateEmptyState(),
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
			State:           GenerateEmptyState(),
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
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				name, symbol, decimals, metadata, totalSupply, owner, err := storage.GetTokenInfoNoController(context.TODO(), m, tokenOneAddress)
				require.NoError(err)
				namesMatch := (string(name) == TokenOneName)
				symbolsMatch := (string(symbol) == TokenOneSymbol)
				decimalsMatch := (decimals == TokenOneDecimals)
				metadataMatch := (string(metadata) == TokenOneMetadata)
				totalSupplyMatch := (totalSupply == uint64(0))
				ownersMatch := (owner == onesAddr)
				return namesMatch && symbolsMatch && decimalsMatch && metadataMatch && totalSupplyMatch && ownersMatch
			},
			Actor: onesAddr,
		},
		{
			Name: "No overwriting existing tokens",
			Action: &CreateToken{
				Name:     []byte(TokenOneName),
				Symbol:   []byte(TokenOneSymbol),
				Metadata: []byte(TokenOneMetadata),
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenAlreadyExists,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Actor: onesAddr,
		},
	}

	chaintest.Run(t, createTokenTests)
}

func TestMintToken(t *testing.T) {
	require := require.New(t)
	ts := tstate.New(1)

	onesAddr, err := createAddressWithSameDigits(1)
	require.NoError(err)

	twosAddr, err := createAddressWithSameDigits(2)
	require.NoError(err)

	mintTokenTests := []chaintest.ActionTest{
		{
			Name: "Mint value must be positive",
			Action: &MintToken{
				To:    onesAddr,
				Token: tokenOneAddress,
				Value: 0,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputMintValueZero,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
		},
		{
			Name: "Only owner can mint",
			Action: &MintToken{
				To:    onesAddr,
				Token: tokenOneAddress,
				Value: InitialTokenMintValue,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenNotOwner,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				mu := chaintest.NewInMemoryStore()
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				require.NoError(storage.SetTokenInfo(context.TODO(), mu, tokenOneAddress, []byte(TokenOneName), []byte(TokenOneSymbol), TokenOneDecimals, []byte(TokenOneMetadata), 0, onesAddr))
				return ts.NewView(stateKeys, mu.Storage)
			}(),
			Actor: twosAddr,
		},
		{
			Name: "Can only mint existing tokens",
			Action: &MintToken{
				To:    onesAddr,
				Token: tokenOneAddress,
				Value: InitialTokenMintValue,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenDoesNotExist,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.Read)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
		},
		{
			Name: "Correct mints can occur",
			Action: &MintToken{
				To:    onesAddr,
				Token: tokenOneAddress,
				Value: InitialTokenMintValue,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     nil,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, onesAddr)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				_, _, _, _, totalSupply, _, err := storage.GetTokenInfoNoController(context.TODO(), m, tokenOneAddress)
				require.NoError(err)
				supplyMatches := (totalSupply == InitialTokenMintValue)
				balance, err := storage.GetTokenAccountNoController(context.TODO(), m, tokenOneAddress, onesAddr)
				require.NoError(err)
				balancesMatch := (balance == InitialTokenMintValue)
				return supplyMatches && balancesMatch
			},
			Actor: onesAddr,
		},
	}

	chaintest.Run(t, mintTokenTests)
}

func TestBurnToken(t *testing.T) {
	require := require.New(t)
	ts := tstate.New(1)

	onesAddr, err := createAddressWithSameDigits(1)
	require.NoError(err)

	burnTokenTests := []chaintest.ActionTest{
		{
			Name: "Can only burn existing tokens",
			Action: &BurnToken{
				TokenAddress: tokenOneAddress,
				Value:        InitialTokenBurnValue,
			},
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.Read)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenDoesNotExist,
		},
		{
			Name: "Burn value must be greater than 0",
			Action: &BurnToken{
				TokenAddress: tokenOneAddress,
				Value:        0,
			},
			State:           GenerateEmptyState(),
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputBurnValueZero,
		},
		{
			Name: "Burn value must be greater or equal to than actor balance",
			Action: &BurnToken{
				TokenAddress: tokenOneAddress,
				Value:        InitialTokenBurnValue,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
			},
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, onesAddr)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputInsufficientTokenBalance,
			Actor:           onesAddr,
		},
		{
			Name: "Correct burns can occur",
			Action: &BurnToken{
				TokenAddress: tokenOneAddress,
				Value:        InitialTokenBurnValue,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&MintToken{
					To:    onesAddr,
					Token: tokenOneAddress,
					Value: InitialTokenMintValue,
				},
			},
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, onesAddr)), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     nil,
			Actor:           onesAddr,
		},
	}

	chaintest.Run(t, burnTokenTests)
}

func TestTransferToken(t *testing.T) {
	require := require.New(t)
	ts := tstate.New(1)

	onesAddr, err := createAddressWithSameDigits(1)
	require.NoError(err)

	twosAddr, err := createAddressWithSameDigits(2)
	require.NoError(err)

	transferTokenTests := []chaintest.ActionTest{
		{
			Name: "Can only transfer existing tokens",
			Action: &TransferToken{
				To:           onesAddr,
				TokenAddress: tokenOneAddress,
				Value:        TokenTransferValue,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTokenDoesNotExist,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.Read)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
		},
		{
			Name: "Tranfer value must be greater than 0",
			Action: &TransferToken{
				To:           onesAddr,
				TokenAddress: tokenOneAddress,
				Value:        0,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputTransferValueZero,
			State:           GenerateEmptyState(),
		},
		{
			Name: "Transfer value must be greater or equal to than sender balance",
			Action: &TransferToken{
				To:           onesAddr,
				TokenAddress: tokenOneAddress,
				Value:        TokenTransferValue,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputInsufficientTokenBalance,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, onesAddr)), state.Read)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Actor: onesAddr,
		},
		{
			Name: "Correct transfers can occur",
			Action: &TransferToken{
				To:           twosAddr,
				TokenAddress: tokenOneAddress,
				Value:        TokenTransferValue,
			},
			SetupActions: []chain.Action{
				&CreateToken{
					Name:     []byte(TokenOneName),
					Symbol:   []byte(TokenOneSymbol),
					Metadata: []byte(TokenOneMetadata),
				},
				&MintToken{
					To:    onesAddr,
					Token: tokenOneAddress,
					Value: InitialTokenMintValue,
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     nil,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(storage.TokenInfoKey(tokenOneAddress)), state.All)
				stateKeys.Add(string(storage.TokenAccountKey(tokenOneAddress, onesAddr)), state.All)
				stateKeys.Add((string(storage.TokenAccountKey(tokenOneAddress, twosAddr))), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				accountOneBalance, err := storage.GetTokenAccountNoController(context.TODO(), m, tokenOneAddress, onesAddr)
				require.NoError(err)
				accountOneBalanceMatches := (accountOneBalance == uint64(0))
				accountTwoBalance, err := storage.GetTokenAccountNoController(context.TODO(), m, tokenOneAddress, twosAddr)
				require.NoError(err)
				accountTwoBalanceMatches := (accountTwoBalance == uint64(1))
				return accountOneBalanceMatches && accountTwoBalanceMatches
			},
			Actor: onesAddr,
		},
	}

	chaintest.Run(t, transferTokenTests)
}

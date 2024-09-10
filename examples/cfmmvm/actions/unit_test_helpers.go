// Copyright (C) 2024, Ava Labs, Inc. All rights reservea.
// See the file LICENSE for licensing terms.

// TODO: rewrite this entire file

package actions

import (
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"
	"github.com/ava-labs/hypersdk/internal/state/tstate"
	"github.com/ava-labs/hypersdk/state"
)

var (
	ts              *tstate.TState
	tokenOneAddress = storage.TokenAddress([]byte(TokenOneName), []byte(TokenOneSymbol), []byte(TokenOneMetadata))
	tokenTwoAddress = storage.TokenAddress([]byte(TokenTwoName), []byte(TokenTwoSymbol), []byte(TokenTwoMetadata))
	lpAddress       = storage.LiquidityPoolAddress(tokenOneAddress, tokenTwoAddress)
	lpTokenAddress  = storage.LiqudityPoolTokenAddress(lpAddress)
)

const (
	TokenOneName     = "HyperSDKCoin" // #nosec G101
	TokenOneSymbol   = "HC"
	TokenOneMetadata = "The coin for HyperSDK" // #nosec G101

	TokenTwoName     = "Avalanche9000"
	TokenTwoSymbol   = "A9000"
	TokenTwoMetadata = "Over 9000"

	TooLargeTokenName     = "Lorem ipsum dolor sit amet, consectetur adipiscing elit pharetra." // #nosec G101
	TooLargeTokenSymbol   = "AAAAAAAAA"
	TooLargeTokenMetadata = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Etiam gravida mauris vitae tortor vehicula dictum. Maecenas rhoncus magna sed justo euismod, eu cursus nunc dapibus. Nunc vestibulum metus sit amet eros pellentesque blandit non at lacus. Ut at donec." // #nosec G101

	InitialTokenMintValue uint64 = 1
	InitialTokenBurnValue uint64 = 1
	TokenTransferValue    uint64 = 1
	InitialSwapValue uint64 = 100

	InitialFee uint64 = 97
)

func GenerateEmptyState() state.Mutable {
	stateKeys := make(state.Keys)
	return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reservea.
// See the file LICENSE for licensing terms.

package actions

import (
	"github.com/ava-labs/hypersdk/chaintest"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
)

var ts *tstate.TState

var tokenOneAddress = storage.TokenAddress([]byte(TokenOneName), []byte(TokenOneSymbol), []byte(TokenOneMetadata))

const (
	TokenOneName     = "HyperSDKCoin" // #nosec G101
	TokenOneSymbol   = "HC"
	TokenOneDecimals = 9
	TokenOneMetadata = "The coin for HyperSDK" // #nosec G101

	TooLargeTokenName     = "Lorem ipsum dolor sit amet, consectetur adipiscing elit pharetra." // #nosec G101
	TooLargeTokenSymbol   = "AAAAAAAAA"
	TooLargeTokenMetadata = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Etiam gravida mauris vitae tortor vehicula dictum. Maecenas rhoncus magna sed justo euismod, eu cursus nunc dapibus. Nunc vestibulum metus sit amet eros pellentesque blandit non at lacus. Ut at donec." // #nosec G101

	InitialTokenMintValue uint64 = 1
	InitialTokenBurnValue uint64 = 1
	TokenTransferValue    uint64 = 1
)

func GenerateEmptyState() state.Mutable {
	stateKeys := make(state.Keys)
	return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
}

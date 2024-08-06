// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/chaintest"
	"github.com/ava-labs/hypersdk/examples/cfmmvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
)

const (
	TokenOneName     = "LuigiCoin"
	TokenOneSymbol   = "LC"
	TokenOneDecimals = 9
	TokenOneMetadata = "A coin that represents Luigi" // #nosec G101

	TokenTwoName     = "Martin"
	TokenTwoSymbol   = "MC"
	TokenTwoDecimals = 9
	TokenTwoMetadata = "A coin that represents Martin" // #nosec G101

	TooLargeTokenName     = "Lorem ipsum dolor sit amet, consectetur adipiscing elit pharetra." // #nosec G101
	TooLargeTokenSymbol   = "AAAAAAAAA"
	TooLargeTokenMetadata = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Etiam gravida mauris vitae tortor vehicula dictum. Maecenas rhoncus magna sed justo euismod, eu cursus nunc dapibus. Nunc vestibulum metus sit amet eros pellentesque blandit non at lacus. Ut at donec." // #nosec G101

	InitialTokenMintValue = 1
	InitialTokenBurnValue = 1
	TokenTransferValue    = 1

	InitialFunctionID = 1
	InitialFee        = 100
	InitialSwapValue  = 100
)

var (
	ts *tstate.TState

	tokenOneAddress = storage.TokenAddress([]byte(TokenOneName), []byte(TokenOneSymbol), TokenOneDecimals, []byte(TokenOneMetadata))
	tokenTwoAddress = storage.TokenAddress([]byte(TokenTwoName), []byte(TokenTwoSymbol), TokenTwoDecimals, []byte(TokenTwoMetadata))
)

func createAddressWithSameDigits(num uint8) (codec.Address, error) {
	addrSlice := make([]byte, codec.AddressLen)
	for i := range addrSlice {
		addrSlice[i] = num
	}
	return codec.ToAddress(addrSlice)
}

func GenerateEmptyState() state.Mutable {
	stateKeys := make(state.Keys)
	return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
}

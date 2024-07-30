// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/nftvm/chaintest"
	"github.com/ava-labs/hypersdk/examples/nftvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
)

const (
	CollectionNameOne     = "The Louvre Museum"
	CollectionSymbolOne   = "LVM"
	CollectionMetadataOne = "The most famous museum in the world"

	InstanceMetadataOne = "Mona Lisa: Leonardo Da Vinci"

	TooLargeCollectionName     = "Lorem ipsum dolor sit amet, consectetur adipiscing elit volutpat." // 65 bytes
	TooLargeCollectionSymbol   = "Avalanche"
	TooLargeCollectionMetadata = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Cras enim enim, vestibulum quis sapien sed, sagittis tempor justo. Sed nec placerat nisi. Suspendisse vitae ligula sed enim maximus ultrices. Sed blandit ac velit tempus volutpat. Cras id nulla donec." // 257 bytes

)

var (
	ts *tstate.TState

	collectionOneAddress  = storage.GenerateNFTCollectionAddress([]byte(CollectionNameOne), []byte(CollectionSymbolOne), []byte(CollectionMetadataOne))
	collectionOneStateKey = storage.CollectionStateKey(collectionOneAddress)

	instanceOneStateKey      = storage.InstanceStateKey(collectionOneAddress, 0)
	instanceOneNum           = uint32(0)
	instanceOneOrderPrice    = uint64(5)
	instanceOneOrderID, _    = storage.GenerateOrderID(collectionOneAddress, instanceOneNum, instanceOneOrderPrice)
	instanceOneOrderStateKey = storage.MarketplaceOrderStateKey(instanceOneOrderID)
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
	stateKeys.Add(string(collectionOneStateKey), state.Read)
	return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
}

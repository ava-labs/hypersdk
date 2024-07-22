package actions

import (
	"context"
	"testing"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/nftvm/chaintest"
	"github.com/ava-labs/hypersdk/examples/nftvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
	"github.com/stretchr/testify/require"
)

func TestNFTCollections(t *testing.T) {
	require := require.New(t)
	ts := tstate.New(1)

	onesAddr, err := createAddressWithSameDigits(1)
	require.NoError(err)

	twosAddr, err := createAddressWithSameDigits(2)
	require.NoError(err)

	// threesAddr, err := createAddressWithSameDigits(3)
	// require.NoError(err)

	nftCollectionTests := []chaintest.ActionTest{
		{
			Name: "Not allowing empty name collections",
			Action: &CreateNFTCollection{
				Name:     []byte{},
				Symbol:   []byte(CollectionSymbolOne),
				Metadata: []byte(CollectionMetadataOne),
			},
			ExpectedErr:     ErrOutputCollectionNameEmpty,
			ExpectedOutputs: [][]uint8(nil),
			State:           GenerateEmptyState(),
		},
		{
			Name: "Not allowing empty symbol collections",
			Action: &CreateNFTCollection{
				Name:     []byte(CollectionNameOne),
				Symbol:   []byte{},
				Metadata: []byte(CollectionMetadataOne),
			},
			ExpectedErr:     ErrOutputCollectionSymbolEmpty,
			ExpectedOutputs: [][]byte(nil),
			State:           GenerateEmptyState(),
		},
		{
			Name: "Not allowing empty metadata collections",
			Action: &CreateNFTCollection{
				Name:     []byte(CollectionNameOne),
				Symbol:   []byte(CollectionSymbolOne),
				Metadata: []byte{},
			},
			ExpectedErr:     ErrOutputCollectionMetadataEmpty,
			ExpectedOutputs: [][]byte(nil),
			State:           GenerateEmptyState(),
		},
		{
			Name: "Not allowing too large collection name",
			Action: &CreateNFTCollection{
				Name:     []byte(TooLargeCollectionName),
				Symbol:   []byte(CollectionSymbolOne),
				Metadata: []byte(CollectionMetadataOne),
			},
			ExpectedErr:     ErrOutputCollectionNameTooLarge,
			ExpectedOutputs: [][]byte(nil),
			State:           GenerateEmptyState(),
		},
		{
			Name: "Not allowing too large symbol name",
			Action: &CreateNFTCollection{
				Name:     []byte(CollectionNameOne),
				Symbol:   []byte(TooLargeCollectionSymbol),
				Metadata: []byte(CollectionMetadataOne),
			},
			ExpectedErr:     ErrOutputCollectionSymbolTooLarge,
			ExpectedOutputs: [][]byte(nil),
			State:           GenerateEmptyState(),
		},
		{
			Name: "Not allowing too large collection metadata",
			Action: &CreateNFTCollection{
				Name:     []byte(CollectionNameOne),
				Symbol:   []byte(CollectionSymbolOne),
				Metadata: []byte(TooLargeCollectionMetadata),
			},
			ExpectedErr:     ErrOutputCollectionMetadataTooLarge,
			ExpectedOutputs: [][]byte(nil),
			State:           GenerateEmptyState(),
		},
		{
			Name: "Correct collection is generated",
			Action: &CreateNFTCollection{
				Name:     []byte(CollectionNameOne),
				Symbol:   []byte(CollectionSymbolOne),
				Metadata: []byte(CollectionMetadataOne),
			},
			ExpectedErr:     nil,
			ExpectedOutputs: [][]byte{collectionOneAddress[:]},
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(collectionOneStateKey), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				name, symbol, metadata, numOfInstances, collectionOwner, err := storage.GetNFTCollectionNoController(context.TODO(), m, collectionOneAddress)
				require.NoError(err)
				namesMatch := (string(name) == CollectionNameOne)
				symbolsMatch := (string(symbol) == CollectionSymbolOne)
				metadataMatch := (string(metadata) == CollectionMetadataOne)
				numOfInstancesMatch := (uint32(0) == numOfInstances)
				collectionOwnerMatch := (collectionOwner == onesAddr)
				return namesMatch && symbolsMatch && metadataMatch && numOfInstancesMatch && collectionOwnerMatch
			},
			Actor: onesAddr,
		},
		{
			Name: "Does not allow overwriting of an existing NFT collection",
			Action: &CreateNFTCollection{
				Name:     []byte(CollectionNameOne),
				Symbol:   []byte(CollectionSymbolOne),
				Metadata: []byte(CollectionMetadataOne),
			},
			ExpectedErr:     ErrOutputNFTCollectionAlreadyExists,
			ExpectedOutputs: [][]byte(nil),
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				store := chaintest.NewInMemoryStore()
				require.NoError(storage.SetNFTCollection(context.TODO(), store, collectionOneAddress, []byte(CollectionNameOne), []byte(CollectionSymbolOne), []byte(CollectionMetadataOne), 0, onesAddr))
				stateKeys.Add(string(collectionOneStateKey), state.All)

				return ts.NewView(stateKeys, store.Storage)
			}(),
		},
		{
			Name: "Only current owner can transfer ownership of collection",
			Action: &TransferNFTCollectionOwnership{
				CollectionAddress: collectionOneAddress,
				To:                codec.EmptyAddress,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputNFTCollectionNotOwner,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				store := chaintest.NewInMemoryStore()
				require.NoError(storage.SetNFTCollection(context.TODO(), store, collectionOneAddress, []byte(CollectionNameOne), []byte(CollectionSymbolOne), []byte(CollectionMetadataOne), 0, onesAddr))
				stateKeys.Add(string(collectionOneStateKey), state.Read)
				return ts.NewView(stateKeys, store.Storage)
			}(),
			Actor: twosAddr,
		},
		{
			Name: "Only transfer of existing collections can occur",
			Action: &TransferNFTCollectionOwnership{
				CollectionAddress: collectionOneAddress,
				To:                codec.EmptyAddress,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     ErrOutputNFTCollectionDoesNotExist,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(collectionOneStateKey), state.Read)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
		},
		{
			Name: "Correct transfer of NFT collection can occur",
			Action: &TransferNFTCollectionOwnership{
				CollectionAddress: collectionOneAddress,
				To:                codec.EmptyAddress,
			},
			SetupActions: []chain.Action{
				&CreateNFTCollection{
					Name:     []byte(CollectionNameOne),
					Symbol:   []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr:     nil,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(collectionOneStateKey), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				name, symbol, metadata, numOfInstances, collectionOwner, err := storage.GetNFTCollectionNoController(context.TODO(), m, collectionOneAddress)
				require.NoError(err)
				namesMatch := (string(name) == CollectionNameOne)
				symbolsMatch := (string(symbol) == CollectionSymbolOne)
				metadataMatch := (string(metadata) == CollectionMetadataOne)
				numOfInstancesMatch := (numOfInstances == uint32(0))
				newOwnersMatch := (codec.EmptyAddress == collectionOwner)
				return namesMatch && symbolsMatch && metadataMatch && numOfInstancesMatch && newOwnersMatch
			},
			Actor: onesAddr,
		},
	}

	chaintest.Run(t, nftCollectionTests)
}

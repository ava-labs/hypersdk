// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/nftvm/chaintest"
	"github.com/ava-labs/hypersdk/examples/nftvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
)

func TestNFTInstances(t *testing.T) {
	require := require.New(t)
	ts := tstate.New(1)

	onesAddr, err := createAddressWithSameDigits(1)
	require.NoError(err)

	twosAddr, err := createAddressWithSameDigits(2)
	require.NoError(err)

	nftInstanceTests := []chaintest.ActionTest{
		{
			Name: "Not allowing instance creation without an existing parent collection",
			Action: &CreateNFTInstance{
				Owner:            onesAddr,
				ParentCollection: collectionOneAddress,
				Metadata:         []byte(InstanceMetadataOne),
			},
			ExpectedErr:     ErrOutputNFTCollectionDoesNotExist,
			ExpectedOutputs: [][]uint8(nil),
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(collectionOneStateKey), state.Read)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
		},
		{
			Name: "Correct instance is generated",
			Action: &CreateNFTInstance{
				Owner:            onesAddr,
				ParentCollection: collectionOneAddress,
				Metadata:         []byte(InstanceMetadataOne),
			},
			SetupActions: []chain.Action{
				&CreateNFTCollection{
					Name:     []byte(CollectionNameOne),
					Symbol:   []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
			},
			ExpectedErr: nil,
			ExpectedOutputs: func() [][]byte {
				v := make([]byte, consts.Uint32Len)
				binary.BigEndian.PutUint32(v, 0)
				return [][]byte{v}
			}(),
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(collectionOneStateKey), state.All)
				stateKeys.Add(string(instanceOneStateKey), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				instanceOwner, metadata, isListedOnMarketplace, err := storage.GetNFTInstanceNoController(context.TODO(), m, collectionOneAddress, 0)
				require.NoError(err)
				instanceOwnersMatch := (instanceOwner == onesAddr)
				metadataMatch := (string(metadata) == InstanceMetadataOne)
				return instanceOwnersMatch && metadataMatch && !isListedOnMarketplace
			},
		},
		{
			Name: "Parent Collection is Updated",
			Action: &CreateNFTInstance{
				Owner:            onesAddr,
				ParentCollection: collectionOneAddress,
				Metadata:         []byte(InstanceMetadataOne),
			},
			SetupActions: []chain.Action{
				&CreateNFTCollection{
					Name:     []byte(CollectionNameOne),
					Symbol:   []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
			},
			ExpectedErr: nil,
			ExpectedOutputs: func() [][]byte {
				v := make([]byte, consts.Uint32Len)
				binary.BigEndian.PutUint32(v, 0)
				return [][]byte{v}
			}(),
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(collectionOneStateKey), state.All)
				stateKeys.Add(string(instanceOneStateKey), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				_, _, _, numOfInstances, _, err := storage.GetNFTCollectionNoController(context.TODO(), m, collectionOneAddress)
				require.NoError(err)
				return numOfInstances == uint32(1)
			},
		},
		{
			Name: "Only instance owner can transfer instances",
			Action: &TransferNFTInstance{
				CollectionAddress: collectionOneAddress,
				InstanceNum:       instanceOneNum,
				To:                codec.EmptyAddress,
			},
			SetupActions: []chain.Action{
				&CreateNFTCollection{
					Name:     []byte(CollectionNameOne),
					Symbol:   []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
				&CreateNFTInstance{
					Owner:            onesAddr,
					ParentCollection: collectionOneAddress,
					Metadata:         []byte(InstanceMetadataOne),
				},
			},
			ExpectedErr:     ErrOutputNFTInstanceNotOwner,
			ExpectedOutputs: [][]byte(nil),
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(collectionOneStateKey), state.All)
				stateKeys.Add(string(instanceOneStateKey), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
		},
		{
			Name: "Correct instance transfer can occur",
			Action: &TransferNFTInstance{
				CollectionAddress: collectionOneAddress,
				InstanceNum:       instanceOneNum,
				To:                codec.EmptyAddress,
			},
			SetupActions: []chain.Action{
				&CreateNFTCollection{
					Name:     []byte(CollectionNameOne),
					Symbol:   []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
				&CreateNFTInstance{
					Owner:            onesAddr,
					ParentCollection: collectionOneAddress,
					Metadata:         []byte(InstanceMetadataOne),
				},
			},
			ExpectedErr:     nil,
			ExpectedOutputs: [][]byte(nil),
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(instanceOneStateKey), state.All)
				stateKeys.Add(string(collectionOneStateKey), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				owner, metadata, isListedOnMarketplace, err := storage.GetNFTInstanceNoController(context.TODO(), m, collectionOneAddress, instanceOneNum)
				require.NoError(err)
				newOwnersMatch := (owner == codec.EmptyAddress)
				metadataMatch := (string(metadata) == InstanceMetadataOne)
				return newOwnersMatch && metadataMatch && !isListedOnMarketplace
			},
			Actor: onesAddr,
		},
		{
			Name: "Only allow transfer of existing instances",
			Action: &TransferNFTInstance{
				CollectionAddress: collectionOneAddress,
				InstanceNum:       instanceOneNum,
				To:                codec.EmptyAddress,
			},
			SetupActions: []chain.Action{
				&CreateNFTCollection{
					Name:     []byte(CollectionNameOne),
					Symbol:   []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
			},
			ExpectedErr:     ErrOutputNFTInstanceNotFound,
			ExpectedOutputs: [][]byte(nil),
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(collectionOneStateKey), state.All)
				stateKeys.Add(string(instanceOneStateKey), state.Read)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
		},
		{
			Name: "Only owner of collection can mint instances",
			Action: &CreateNFTInstance{
				Owner:            twosAddr,
				ParentCollection: collectionOneAddress,
				Metadata:         []byte(InstanceMetadataOne),
			},
			ExpectedErr:     ErrOutputNFTCollectionNotOwner,
			ExpectedOutputs: [][]byte(nil),
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				mu := chaintest.NewInMemoryStore()
				stateKeys.Add(string(collectionOneStateKey), state.All)
				stateKeys.Add(string(instanceOneStateKey), state.Read)

				require.NoError(storage.SetNFTCollection(context.TODO(), mu, collectionOneAddress, []byte(CollectionNameOne), []byte(CollectionSymbolOne), []byte(CollectionMetadataOne), 0, twosAddr))
				return ts.NewView(stateKeys, mu.Storage)
			}(),
			Actor: onesAddr,
		},
	}

	chaintest.Run(t, nftInstanceTests)
}

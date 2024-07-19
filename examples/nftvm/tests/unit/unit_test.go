package unit_test

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/nftvm/actions"
	"github.com/ava-labs/hypersdk/examples/nftvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
	"github.com/stretchr/testify/require"
)

const (
	CollectionNameOne = "The Louvre Museum"
	CollectionSymbolOne = "LVM"
	CollectionMetadataOne = "The most famous museum in the world"

	InstanceMetadataOne = "Mona Lisa: Leonardo Da Vinci"

	TooLargeCollectionName = "Lorem ipsum dolor sit amet, consectetur adipiscing elit volutpat." // 65 bytes
	TooLargeCollectionSymbol = "Avalanche"
	TooLargeCollectionMetadata = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Cras enim enim, vestibulum quis sapien sed, sagittis tempor justo. Sed nec placerat nisi. Suspendisse vitae ligula sed enim maximus ultrices. Sed blandit ac velit tempus volutpat. Cras id nulla donec." // 257 bytes

	TooLargeInstanceMetadata = ""
)

var (
	ts *tstate.TState

	collectionOneAddress = storage.GenerateNFTCollectionAddress([]byte(CollectionNameOne), []byte(CollectionSymbolOne), []byte(CollectionMetadataOne))
	collectionOneStateKey = storage.CollectionStateKey(collectionOneAddress)

	instanceOneStateKey = storage.InstanceStateKey(collectionOneAddress, 0)
	instanceOneNum = uint32(0)
	instanceOneOrderPrice = uint64(5)
	instanceOneOrderID, _ = storage.GenerateOrderID(collectionOneAddress, instanceOneNum, instanceOneOrderPrice)
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
	return ts.NewView(stateKeys, NewInMemoryStore().Storage)
}

func TestNFTCollections(t *testing.T) {

	require := require.New(t)
	ts := tstate.New(1)

	onesAddr, err := createAddressWithSameDigits(1)
	require.NoError(err)

	twosAddr, err := createAddressWithSameDigits(2)
	require.NoError(err)

	// threesAddr, err := createAddressWithSameDigits(3)
	// require.NoError(err)

	nftCollectionInvariantTests := []ActionTest{
		{
			Name: "Not allowing empty name collections",
			Action: &actions.CreateNFTCollection{
					Name: []byte{},
					Symbol: []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
			ExpectedErr: actions.ErrOutputCollectionNameEmpty,
			ExpectedOutputs: [][]uint8(nil),
			State: GenerateEmptyState(),
		},
		{
			Name: "Not allowing empty symbol collections",
			Action: &actions.CreateNFTCollection{
						Name: []byte(CollectionNameOne),
						Symbol: []byte{},
						Metadata: []byte(CollectionMetadataOne),
				},
			ExpectedErr: actions.ErrOutputCollectionSymbolEmpty,
			ExpectedOutputs: [][]byte(nil),
			State: GenerateEmptyState(),
		},
		{
			Name: "Not allowing empty metadata collections",
			Action: &actions.CreateNFTCollection{
						Name: []byte(CollectionNameOne),
						Symbol: []byte(CollectionSymbolOne),
						Metadata: []byte{},
					},
			ExpectedErr: actions.ErrOutputCollectionMetadataEmpty,
			ExpectedOutputs: [][]byte(nil),
			State: GenerateEmptyState(),
		},
		{
			Name: "Not allowing too large collection name",
			Action: &actions.CreateNFTCollection{
					Name: []byte(TooLargeCollectionName),
					Symbol: []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
			ExpectedErr: actions.ErrOutputCollectionNameTooLarge,
			ExpectedOutputs: [][]byte(nil),
			State: GenerateEmptyState(),
		},
		{
			Name: "Not allowing too large symbol name",
			Action: &actions.CreateNFTCollection{
						Name: []byte(CollectionNameOne),
						Symbol: []byte(TooLargeCollectionSymbol),
						Metadata: []byte(CollectionMetadataOne),
					},
			ExpectedErr: actions.ErrOutputCollectionSymbolTooLarge,
			ExpectedOutputs: [][]byte(nil),
			State: GenerateEmptyState(),
		},
		{
			Name: "Not allowing too large collection metadata",
			Action: &actions.CreateNFTCollection{
					Name: []byte(CollectionNameOne),
					Symbol: []byte(CollectionSymbolOne),
					Metadata: []byte(TooLargeCollectionMetadata),
				},
			ExpectedErr: actions.ErrOutputCollectionMetadataTooLarge,
			ExpectedOutputs: [][]byte(nil),
			State: GenerateEmptyState(),
		},
		{
			Name: "Correct collection is generated",
			Action: &actions.CreateNFTCollection{
				Name: []byte(CollectionNameOne),
				Symbol: []byte(CollectionSymbolOne),
				Metadata: []byte(CollectionMetadataOne),
			},
			ExpectedErr: nil,
			ExpectedOutputs: [][]byte{collectionOneAddress[:]},
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(collectionOneStateKey), state.All)
				return ts.NewView(stateKeys, NewInMemoryStore().Storage)
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
			Action: &actions.CreateNFTCollection{
				Name: []byte(CollectionNameOne),
				Symbol: []byte(CollectionSymbolOne),
				Metadata: []byte(CollectionMetadataOne),
			},
			ExpectedErr: actions.ErrOutputNFTCollectionAlreadyExists,
			ExpectedOutputs: [][]byte(nil),
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				store := NewInMemoryStore()
				require.NoError(storage.SetNFTCollection(context.TODO(), store, collectionOneAddress, []byte(CollectionNameOne), []byte(CollectionSymbolOne), []byte(CollectionMetadataOne), 0, onesAddr))
				stateKeys.Add(string(collectionOneStateKey), state.All)

				return ts.NewView(stateKeys, store.Storage)
			}(),
		},
		{
			Name: "Only current owner can transfer ownership of collection",
			Action: &actions.TransferNFTCollectionOwnership{
				CollectionAddress: collectionOneAddress,
				To: codec.EmptyAddress,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: actions.ErrOutputNFTCollectionNotOwner,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				store := NewInMemoryStore()
				require.NoError(storage.SetNFTCollection(context.TODO(), store, collectionOneAddress, []byte(CollectionNameOne), []byte(CollectionSymbolOne), []byte(CollectionMetadataOne), 0, onesAddr))
				stateKeys.Add(string(collectionOneStateKey), state.Read)
				return ts.NewView(stateKeys, store.Storage)
			}(),
			Actor: twosAddr,
		},
		{
			Name: "Only transfer of existing collections can occur",
			Action: &actions.TransferNFTCollectionOwnership{
				CollectionAddress: collectionOneAddress,
				To: codec.EmptyAddress,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: actions.ErrOutputNFTCollectionDoesNotExist,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(collectionOneStateKey), state.Read)
				return ts.NewView(stateKeys, NewInMemoryStore().Storage)
			}(),
		},
		{
			Name: "Correct transfer of NFT collection can occur",
			Action: &actions.TransferNFTCollectionOwnership{
				CollectionAddress: collectionOneAddress,
				To: codec.EmptyAddress,
			},
			SetupActions: []chain.Action{
				&actions.CreateNFTCollection{
					Name: []byte(CollectionNameOne),
					Symbol: []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: nil,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(collectionOneStateKey), state.All)
				return ts.NewView(stateKeys, NewInMemoryStore().Storage)
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

	Run(t, nftCollectionInvariantTests)


	nftInstanceTests := []ActionTest{
		{
			Name: "Not allowing instance creation without an existing parent collection",
			Action: &actions.CreateNFTInstance{
						Owner: onesAddr,
						ParentCollection: collectionOneAddress,
						Metadata: []byte(InstanceMetadataOne),
					},
			ExpectedErr: actions.ErrOutputNFTCollectionDoesNotExist,
			ExpectedOutputs: [][]uint8(nil),
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(collectionOneStateKey), state.Read)
				return ts.NewView(stateKeys, NewInMemoryStore().Storage)
			}(),
		},
		{
			Name: "Correct instance is generated",
			Action: &actions.CreateNFTInstance{
						Owner: onesAddr,
						ParentCollection: collectionOneAddress,
						Metadata: []byte(InstanceMetadataOne),
					},
			SetupActions: []chain.Action{
				&actions.CreateNFTCollection{
					Name: []byte(CollectionNameOne),
					Symbol: []byte(CollectionSymbolOne),
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
				return ts.NewView(stateKeys, NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				instanceOwner, metadata, err := storage.GetNFTInstanceNoController(context.TODO(), m, collectionOneAddress, 0)
				require.NoError(err)
				instanceOwnersMatch := (instanceOwner == onesAddr)
				metadataMatch := (string(metadata) == InstanceMetadataOne)
				return instanceOwnersMatch && metadataMatch
			},
		},
		{
			Name: "Parent Collection is Updated",
			Action: &actions.CreateNFTInstance{
				Owner: onesAddr,
				ParentCollection: collectionOneAddress,
				Metadata: []byte(InstanceMetadataOne),
			},
			SetupActions: []chain.Action{
				&actions.CreateNFTCollection{
					Name: []byte(CollectionNameOne),
					Symbol: []byte(CollectionSymbolOne),
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
				return ts.NewView(stateKeys, NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				_, _, _, numOfInstances, _, err := storage.GetNFTCollectionNoController(context.TODO(), m, collectionOneAddress)
				require.NoError(err)
				return numOfInstances == uint32(1)
			},
		},
		// {
		// 	Name: "Only owner can transfer instances",
		// },
		{
			Name: "Correct instance transfer can occur",
			Action: &actions.TransferNFTInstance{
				CollectionAddress: collectionOneAddress,
				InstanceNum: instanceOneNum,
				To: codec.EmptyAddress,
			},
			SetupActions: []chain.Action {
				&actions.CreateNFTCollection{
					Name: []byte(CollectionNameOne),
					Symbol: []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
				&actions.CreateNFTInstance{
					Owner: onesAddr,
					ParentCollection: collectionOneAddress,
					Metadata: []byte(InstanceMetadataOne),
				},
			},
			ExpectedErr: nil,
			ExpectedOutputs: [][]byte(nil),
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(instanceOneStateKey), state.All)
				stateKeys.Add(string(collectionOneStateKey), state.All)
				return ts.NewView(stateKeys, NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				owner, metadata, err := storage.GetNFTInstanceNoController(context.TODO(), m, collectionOneAddress, instanceOneNum)
				require.NoError(err)
				newOwnersMatch := (owner == codec.EmptyAddress)
				metadataMatch := (string(metadata) == InstanceMetadataOne)
				return newOwnersMatch && metadataMatch
			},
			Actor: onesAddr,
		},
	}

	Run(t, nftInstanceTests)

	marketplaceOrderTests := []ActionTest{
		{
			Name: "No free instance orders",
			Action: &actions.CreateMarketplaceOrder{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				Price: uint64(0),
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: actions.ErrOutputMarketplaceOrderPriceZero,
			State: GenerateEmptyState(),
		},
		{
			Name: "Only owners are allowed to sell their NFT instances",
			Action: &actions.CreateMarketplaceOrder{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				Price: instanceOneOrderPrice,
			},
			SetupActions: []chain.Action{
				&actions.CreateNFTCollection{
					Name: []byte(CollectionNameOne),
					Symbol: []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
				&actions.CreateNFTInstance{
					Owner: onesAddr,
					ParentCollection: collectionOneAddress,
					Metadata: []byte(InstanceMetadataOne),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: actions.ErrOutputMarketplaceOrderNotOwner,
			Actor: twosAddr,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(collectionOneStateKey), state.All)
				stateKeys.Add(string(instanceOneStateKey), state.All)
				stateKeys.Add(string(instanceOneOrderStateKey), state.All)
				return ts.NewView(stateKeys, NewInMemoryStore().Storage)
			}(),
		},
		{
			Name: "Correct orders are placed",
			Action: &actions.CreateMarketplaceOrder{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				Price: instanceOneOrderPrice,
			},
			Actor: onesAddr,
			SetupActions: []chain.Action{
				&actions.CreateNFTCollection{
					Name: []byte(CollectionNameOne),
					Symbol: []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
				&actions.CreateNFTInstance{
					Owner: onesAddr,
					ParentCollection: collectionOneAddress,
					Metadata: []byte(InstanceMetadataOne),
				},
			},
			ExpectedOutputs: [][]byte{instanceOneOrderID[:]},
			ExpectedErr: nil,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(collectionOneStateKey), state.All)
				stateKeys.Add(string(instanceOneStateKey), state.All)
				stateKeys.Add(string(instanceOneOrderStateKey), state.All)
				return ts.NewView(stateKeys, NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				price, err := storage.GetMarketplaceOrderNoController(context.TODO(), m, instanceOneOrderID)
				require.NoError(err)
				return price == instanceOneOrderPrice
			},
		},
		{
			Name: "Correct purchases are allowed",
			Action: &actions.BuyNFT{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				OrderID: instanceOneOrderID,
				CurrentOwner: onesAddr,
			},
			SetupActions: []chain.Action{
				&actions.CreateNFTCollection{
					Name: []byte(CollectionNameOne),
					Symbol: []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
				&actions.CreateNFTInstance{
					Owner: onesAddr,
					ParentCollection: collectionOneAddress,
					Metadata: []byte(InstanceMetadataOne),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: nil,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				st := NewInMemoryStore()
				stateKeys.Add(string(collectionOneStateKey), state.All)
				stateKeys.Add(string(instanceOneStateKey), state.All)
				stateKeys.Add(string(instanceOneOrderStateKey), state.All)
				stateKeys.Add(string(storage.BalanceKey(onesAddr)), state.All)
				stateKeys.Add(string(storage.BalanceKey(twosAddr)), state.All)
				require.NoError(storage.SetBalance(context.TODO(), st, twosAddr, 2000))
				_, err := storage.CreateMarketplaceOrder(context.TODO(), st, collectionOneAddress, instanceOneNum, instanceOneOrderPrice)
				require.NoError(err)
				return ts.NewView(stateKeys, st.Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				// Assert balances and instance ownership
				buyerBalance, err := storage.GetBalance(context.TODO(), m, twosAddr)
				require.NoError(err)
				sellerBalance, err := storage.GetBalance(context.TODO(), m, onesAddr)
				require.NoError(err)
				newOwner, metadata, err := storage.GetNFTInstanceNoController(context.TODO(), m, collectionOneAddress, instanceOneNum)
				require.NoError(err)
				return (buyerBalance == 1995) && (sellerBalance == 5) && (newOwner == twosAddr) && (string(metadata) == InstanceMetadataOne)
			},
			Actor: twosAddr,
		},
		{
			Name: "Handle nonexistent orders",
			Action: &actions.BuyNFT{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				OrderID: instanceOneOrderID,
				CurrentOwner: onesAddr,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: actions.ErrOutputMarketplaceOrderNotFound,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				st := NewInMemoryStore()
				stateKeys.Add(string(instanceOneOrderStateKey), state.All)
				return ts.NewView(stateKeys, st.Storage)
			}(),
		},
		{
			Name: "Handle nonexistent instances",
			Action: &actions.BuyNFT{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				OrderID: instanceOneOrderID,
				CurrentOwner: onesAddr,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: actions.ErrOutputNFTInstanceNotFound,
			State: func () state.Mutable {
				stateKeys := make(state.Keys)
				st := NewInMemoryStore()
				_, err := storage.CreateMarketplaceOrder(context.TODO(), st, collectionOneAddress, instanceOneNum, instanceOneOrderPrice)
				require.NoError(err)
				stateKeys.Add(string(instanceOneOrderStateKey), state.All)
				return ts.NewView(stateKeys, st.Storage)
			}(),
		},
		{
			Name: "Handle owner inconsistencies",
			Action: &actions.BuyNFT{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				OrderID: instanceOneOrderID,
				CurrentOwner: twosAddr,
			},
			SetupActions: []chain.Action{
				&actions.CreateNFTCollection{
					Name: []byte(CollectionNameOne),
					Symbol: []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
				&actions.CreateNFTInstance{
					Owner: onesAddr,
					ParentCollection: collectionOneAddress,
					Metadata: []byte(InstanceMetadataOne),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: actions.ErrOutputMarketplaceOrderOwnerInconsistency,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				st := NewInMemoryStore()
				stateKeys.Add(string(collectionOneStateKey), state.All)
				stateKeys.Add(string(instanceOneStateKey), state.All)
				stateKeys.Add(string(instanceOneOrderStateKey), state.All)
				_, err := storage.CreateMarketplaceOrder(context.TODO(), st, collectionOneAddress, instanceOneNum, instanceOneOrderPrice)
				require.NoError(err)
				return ts.NewView(stateKeys, st.Storage)
			}(),
		},
		{
			Name: "Handle insufficient buyer balance",
			Action: &actions.BuyNFT{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				OrderID: instanceOneOrderID,
				CurrentOwner: onesAddr,
			},
			SetupActions: []chain.Action{
				&actions.CreateNFTCollection{
					Name: []byte(CollectionNameOne),
					Symbol: []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
				&actions.CreateNFTInstance{
					Owner: onesAddr,
					ParentCollection: collectionOneAddress,
					Metadata: []byte(InstanceMetadataOne),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: actions.ErrOutputMarketplaceOrderInsufficientBalance,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				st := NewInMemoryStore()
				stateKeys.Add(string(collectionOneStateKey), state.All)
				stateKeys.Add(string(instanceOneStateKey), state.All)
				stateKeys.Add(string(instanceOneOrderStateKey), state.All)
				stateKeys.Add(string(storage.BalanceKey(onesAddr)), state.All)
				stateKeys.Add(string(storage.BalanceKey(twosAddr)), state.All)
				_, err := storage.CreateMarketplaceOrder(context.TODO(), st, collectionOneAddress, instanceOneNum, instanceOneOrderPrice)
				require.NoError(err)
				return ts.NewView(stateKeys, st.Storage)
			}(),
			Actor: twosAddr,
		},
	}

	Run(t, marketplaceOrderTests)

}	
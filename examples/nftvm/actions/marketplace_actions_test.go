package actions

import (
	"context"
	"testing"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/nftvm/chaintest"
	"github.com/ava-labs/hypersdk/examples/nftvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
	"github.com/stretchr/testify/require"
)

func TestMarketplace(t *testing.T) {

	require := require.New(t)
	ts := tstate.New(1)

	onesAddr, err := createAddressWithSameDigits(1)
	require.NoError(err)

	twosAddr, err := createAddressWithSameDigits(2)
	require.NoError(err)

	marketplaceTests := []chaintest.ActionTest{
		{
			Name: "No free instance orders",
			Action: &CreateMarketplaceOrder{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				Price: uint64(0),
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: ErrOutputMarketplaceOrderPriceZero,
			State: GenerateEmptyState(),
		},
		{
			Name: "Only owners are allowed to sell their NFT instances",
			Action: &CreateMarketplaceOrder{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				Price: instanceOneOrderPrice,
			},
			SetupActions: []chain.Action{
				&CreateNFTCollection{
					Name: []byte(CollectionNameOne),
					Symbol: []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
				&CreateNFTInstance{
					Owner: onesAddr,
					ParentCollection: collectionOneAddress,
					Metadata: []byte(InstanceMetadataOne),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: ErrOutputMarketplaceOrderNotOwner,
			Actor: twosAddr,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				stateKeys.Add(string(collectionOneStateKey), state.All)
				stateKeys.Add(string(instanceOneStateKey), state.All)
				stateKeys.Add(string(instanceOneOrderStateKey), state.All)
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
		},
		{
			Name: "Correct orders are placed",
			Action: &CreateMarketplaceOrder{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				Price: instanceOneOrderPrice,
			},
			Actor: onesAddr,
			SetupActions: []chain.Action{
				&CreateNFTCollection{
					Name: []byte(CollectionNameOne),
					Symbol: []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
				&CreateNFTInstance{
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
				return ts.NewView(stateKeys, chaintest.NewInMemoryStore().Storage)
			}(),
			Assertion: func(m state.Mutable) bool {
				price, err := storage.GetMarketplaceOrderNoController(context.TODO(), m, instanceOneOrderID)
				require.NoError(err)
				return price == instanceOneOrderPrice
			},
		},
		{
			Name: "Correct purchases are allowed",
			Action: &BuyNFT{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				OrderID: instanceOneOrderID,
				CurrentOwner: onesAddr,
			},
			SetupActions: []chain.Action{
				&CreateNFTCollection{
					Name: []byte(CollectionNameOne),
					Symbol: []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
				&CreateNFTInstance{
					Owner: onesAddr,
					ParentCollection: collectionOneAddress,
					Metadata: []byte(InstanceMetadataOne),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: nil,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				st := chaintest.NewInMemoryStore()
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
			Action: &BuyNFT{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				OrderID: instanceOneOrderID,
				CurrentOwner: onesAddr,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: ErrOutputMarketplaceOrderNotFound,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				st := chaintest.NewInMemoryStore()
				stateKeys.Add(string(instanceOneOrderStateKey), state.All)
				return ts.NewView(stateKeys, st.Storage)
			}(),
		},
		{
			Name: "Handle nonexistent instances",
			Action: &BuyNFT{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				OrderID: instanceOneOrderID,
				CurrentOwner: onesAddr,
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: ErrOutputNFTInstanceNotFound,
			State: func () state.Mutable {
				stateKeys := make(state.Keys)
				st := chaintest.NewInMemoryStore()
				_, err := storage.CreateMarketplaceOrder(context.TODO(), st, collectionOneAddress, instanceOneNum, instanceOneOrderPrice)
				require.NoError(err)
				stateKeys.Add(string(instanceOneOrderStateKey), state.All)
				return ts.NewView(stateKeys, st.Storage)
			}(),
		},
		{
			Name: "Handle owner inconsistencies",
			Action: &BuyNFT{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				OrderID: instanceOneOrderID,
				CurrentOwner: twosAddr,
			},
			SetupActions: []chain.Action{
				&CreateNFTCollection{
					Name: []byte(CollectionNameOne),
					Symbol: []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
				&CreateNFTInstance{
					Owner: onesAddr,
					ParentCollection: collectionOneAddress,
					Metadata: []byte(InstanceMetadataOne),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: ErrOutputMarketplaceOrderOwnerInconsistency,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				st := chaintest.NewInMemoryStore()
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
			Action: &BuyNFT{
				ParentCollection: collectionOneAddress,
				InstanceNum: instanceOneNum,
				OrderID: instanceOneOrderID,
				CurrentOwner: onesAddr,
			},
			SetupActions: []chain.Action{
				&CreateNFTCollection{
					Name: []byte(CollectionNameOne),
					Symbol: []byte(CollectionSymbolOne),
					Metadata: []byte(CollectionMetadataOne),
				},
				&CreateNFTInstance{
					Owner: onesAddr,
					ParentCollection: collectionOneAddress,
					Metadata: []byte(InstanceMetadataOne),
				},
			},
			ExpectedOutputs: [][]byte(nil),
			ExpectedErr: ErrOutputMarketplaceOrderInsufficientBalance,
			State: func() state.Mutable {
				stateKeys := make(state.Keys)
				st := chaintest.NewInMemoryStore()
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
	chaintest.Run(t, marketplaceTests)

}
package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/nftvm/consts"
	"github.com/ava-labs/hypersdk/examples/nftvm/storage"
	"github.com/ava-labs/hypersdk/state"

	lconsts "github.com/ava-labs/hypersdk/consts"
)

var _ chain.Action = (*BuyNFT)(nil)

// TODO: make buy orders more precise to avoid issues such as purchasing stale orders
type BuyNFT struct {
	ParentCollection codec.Address `json:"parentCollection"`

	InstanceNum uint32 `json:"instanceNum"`

	OrderID ids.ID `json:"orderID"`

	CurrentOwner codec.Address `json:"currentOwner"`
}

// ComputeUnits implements chain.Action.
func (b *BuyNFT) ComputeUnits(chain.Rules) uint64 {
	return BuyNFTComputeUnits
}

// Execute implements chain.Action.
func (b *BuyNFT) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) ([][]byte, error) {
	// Check that order exists
	price, err := storage.GetMarketplaceOrderNoController(ctx, mu, b.OrderID)
	if err != nil {
		return nil, ErrOutputMarketplaceOrderNotFound
	}

	// Order exists, confirm owners are same
	owner, metadata, isListedOnMarketplace, err := storage.GetNFTInstanceNoController(ctx, mu, b.ParentCollection, b.InstanceNum)
	if err != nil {
		return nil, ErrOutputNFTInstanceNotFound
	}
	if !isListedOnMarketplace {
		return nil, ErrOutputMarketplaceOrderInstanceNotListed
	}
	if owner != b.CurrentOwner {
		return nil, ErrOutputMarketplaceOrderOwnerInconsistency
	}

	// Subtract balance from buyer
	if err := storage.SubBalance(ctx, mu, actor, price); err != nil {
		return nil, ErrOutputMarketplaceOrderInsufficientBalance
	}

	// Add balance to seller
	if err := storage.AddBalance(ctx, mu, owner, price, true); err != nil {
		return nil, err
	}

	// Finally, transfer NFT instance by modifying state
	// Marketplace indicator for instance is set to false
	if err := storage.SetNFTInstance(ctx, mu, b.ParentCollection, b.InstanceNum, actor, metadata, false); err != nil {
		return nil, err
	}

	return nil, nil
}

// GetTypeID implements chain.Action.
func (b *BuyNFT) GetTypeID() uint8 {
	return consts.BuyNFT
}

// Size implements chain.Action.
func (b *BuyNFT) Size() int {
	return codec.AddressLen + lconsts.Uint32Len + ids.IDLen + codec.AddressLen
}

// StateKeys implements chain.Action.
func (b *BuyNFT) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	// We need to access the following items: the order, the NFT instance, the
	// balance of the buyer, the balance of the seller
	orderStateKey := storage.MarketplaceOrderStateKey(b.OrderID)
	nftInstanceStateKey := storage.InstanceStateKey(b.ParentCollection, b.InstanceNum)
	return state.Keys{
		string(orderStateKey):                      state.All,
		string(nftInstanceStateKey):                state.All,
		string(storage.BalanceKey(actor)):          state.All,
		string(storage.BalanceKey(b.CurrentOwner)): state.All,
	}
}

// StateKeysMaxChunks implements chain.Action.
func (b *BuyNFT) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.BuyNFTStateChunks}
}

// ValidRange implements chain.Action.
func (b *BuyNFT) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

// Marshal implements chain.Action.
func (b *BuyNFT) Marshal(p *codec.Packer) {
	p.PackAddress(b.ParentCollection)
	p.PackUint64(uint64(b.InstanceNum))
	p.PackID(b.OrderID)
	p.PackAddress(b.CurrentOwner)
}

func UnmarshalBuyNFT(p *codec.Packer) (chain.Action, error) {
	var buyNFT BuyNFT
	p.UnpackAddress(&buyNFT.ParentCollection)
	buyNFT.InstanceNum = uint32(p.UnpackUint64(false))
	p.UnpackID(false, &buyNFT.OrderID)
	p.UnpackAddress(&buyNFT.CurrentOwner)

	return &buyNFT, p.Err()
}

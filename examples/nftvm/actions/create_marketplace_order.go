// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

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

var _ chain.Action = (*CreateMarketplaceOrder)(nil)

type CreateMarketplaceOrder struct {
	ParentCollection codec.Address `json:"parentCollection"`

	InstanceNum uint32 `json:"instanceNum"`

	Price uint64 `json:"price"`
}

// ComputeUnits implements chain.Action.
func (c *CreateMarketplaceOrder) ComputeUnits(chain.Rules) uint64 {
	return CreateMarketplaceOrderComputeUnits
}

// Execute implements chain.Action.
func (c *CreateMarketplaceOrder) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) ([][]byte, error) {
	// Enforce order invariants
	if c.Price == 0 {
		return nil, ErrOutputMarketplaceOrderPriceZero
	}

	// Check that instance exists and that actor is owner
	owner, metadata, isListedOnMarketplace, err := storage.GetNFTInstanceNoController(ctx, mu, c.ParentCollection, c.InstanceNum)
	if err != nil {
		return nil, ErrOutputNFTInstanceNotFound
	}
	if owner != actor {
		return nil, ErrOutputMarketplaceOrderNotOwner
	}
	if isListedOnMarketplace {
		return nil, ErrOutputMarketplaceOrderInstanceAlreadyListed
	}

	// Create order
	orderID, err := storage.CreateMarketplaceOrder(ctx, mu, c.ParentCollection, c.InstanceNum, c.Price)
	if err != nil {
		return nil, ErrOutputMarketplaceOrderAlreadyExists
	}
	// Update instance marketplace identifier
	if err = storage.SetNFTInstance(ctx, mu, c.ParentCollection, c.InstanceNum, owner, metadata, true); err != nil {
		return nil, err
	}

	return [][]byte{orderID[:]}, nil
}

// GetTypeID implements chain.Action.
func (c *CreateMarketplaceOrder) GetTypeID() uint8 {
	return consts.CreateMarketplaceOrder
}

// Size implements chain.Action.
func (c *CreateMarketplaceOrder) Size() int {
	return codec.AddressLen + lconsts.Uint32Len + lconsts.Uint64Len
}

// StateKeys implements chain.Action.
func (c *CreateMarketplaceOrder) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	// We need to access the states of both the potential order and of the assoc instance
	instanceStateKey := storage.InstanceStateKey(c.ParentCollection, c.InstanceNum)
	potentialOrderID, _ := storage.GenerateOrderID(c.ParentCollection, c.InstanceNum, c.Price)
	orderStateKey := storage.MarketplaceOrderStateKey(potentialOrderID)
	return state.Keys{
		string(instanceStateKey): state.All,
		string(orderStateKey):    state.All,
	}
}

// StateKeysMaxChunks implements chain.Action.
func (c *CreateMarketplaceOrder) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.MarketplaceOrderStateChunks}
}

// ValidRange implements chain.Action.
func (c *CreateMarketplaceOrder) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

// Marshal implements chain.Action.
func (c *CreateMarketplaceOrder) Marshal(p *codec.Packer) {
	p.PackAddress(c.ParentCollection)
	p.PackUint64(uint64(c.InstanceNum))
	p.PackUint64(c.Price)
}

func UnmarshalCreateMarketplaceOrder(p *codec.Packer) (chain.Action, error) {
	var createMarketplaceOrder CreateMarketplaceOrder

	p.UnpackAddress(&createMarketplaceOrder.ParentCollection)
	createMarketplaceOrder.InstanceNum = uint32(p.UnpackUint64(false))
	createMarketplaceOrder.Price = p.UnpackUint64(false)

	return &createMarketplaceOrder, p.Err()
}

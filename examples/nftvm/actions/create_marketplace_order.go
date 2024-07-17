package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.Action = (*CreateMarketplaceOrder)(nil)

type CreateMarketplaceOrder struct {
	ParentCollection codec.Address

	InstanceNum uint32

	Price uint64
}

// ComputeUnits implements chain.Action.
func (c *CreateMarketplaceOrder) ComputeUnits(chain.Rules) uint64 {
	panic("unimplemented")
}

// Execute implements chain.Action.
func (c *CreateMarketplaceOrder) Execute(ctx context.Context, r chain.Rules, mu state.Mutable, timestamp int64, actor codec.Address, actionID ids.ID) (outputs [][]byte, err error) {
	panic("unimplemented")
}

// GetTypeID implements chain.Action.
func (c *CreateMarketplaceOrder) GetTypeID() uint8 {
	panic("unimplemented")
}

// Marshal implements chain.Action.
func (c *CreateMarketplaceOrder) Marshal(p *codec.Packer) {
	panic("unimplemented")
}

// Size implements chain.Action.
func (c *CreateMarketplaceOrder) Size() int {
	panic("unimplemented")
}

// StateKeys implements chain.Action.
func (c *CreateMarketplaceOrder) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	panic("unimplemented")
}

// StateKeysMaxChunks implements chain.Action.
func (c *CreateMarketplaceOrder) StateKeysMaxChunks() []uint16 {
	panic("unimplemented")
}

// ValidRange implements chain.Action.
func (c *CreateMarketplaceOrder) ValidRange(chain.Rules) (start int64, end int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

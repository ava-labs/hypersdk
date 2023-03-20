// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
)

var _ chain.Rules = (*Rules)(nil)

type Rules struct {
	g *Genesis
}

// TODO: use upgradeBytes
func (g *Genesis) Rules(int64) *Rules {
	return &Rules{g}
}

func (*Rules) GetWarpConfig(ids.ID) (bool, uint64, uint64) {
	// We allow inbound transfers from all sources as long as 80% of stake has
	// signed a message.
	//
	// This is safe because the tokenvm scopes all assets by their source chain.
	return true, 4, 5
}

func (r *Rules) GetWarpBaseFee() uint64 {
	return r.g.WarpBaseFee
}

func (r *Rules) GetWarpFeePerSigner() uint64 {
	return r.g.WarpFeePerSigner
}

func (r *Rules) GetMaxBlockTxs() int {
	return r.g.MaxBlockTxs
}

func (r *Rules) GetValidityWindow() int64 {
	return r.g.ValidityWindow
}

func (r *Rules) GetMaxBlockUnits() uint64 {
	return r.g.MaxBlockUnits
}

func (r *Rules) GetBaseUnits() uint64 {
	return r.g.BaseUnits
}

func (r *Rules) GetMinUnitPrice() uint64 {
	return r.g.MinUnitPrice
}

func (r *Rules) GetUnitPriceChangeDenominator() uint64 {
	return r.g.UnitPriceChangeDenominator
}

func (r *Rules) GetWindowTargetUnits() uint64 {
	return r.g.WindowTargetUnits
}

func (r *Rules) GetMinBlockCost() uint64 {
	return r.g.MinBlockCost
}

func (r *Rules) GetBlockCostChangeDenominator() uint64 {
	return r.g.BlockCostChangeDenominator
}

func (r *Rules) GetWindowTargetBlocks() uint64 {
	return r.g.WindowTargetBlocks
}

func (*Rules) FetchCustom(string) (any, bool) {
	return nil, false
}

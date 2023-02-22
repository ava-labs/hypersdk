// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
)

var _ chain.Rules = (*Rules)(nil)

type Rules struct {
	chainID ids.ID
	g       *Genesis
}

// TODO: use upgradeBytes
func (g *Genesis) Rules(chainID ids.ID, _ int64) *Rules {
	return &Rules{chainID, g}
}

func (r *Rules) GetChainID() ids.ID {
	return r.chainID
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

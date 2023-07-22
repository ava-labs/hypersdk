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

	networkID uint32
	chainID   ids.ID
}

// TODO: use upgradeBytes
func (g *Genesis) Rules(_ int64, networkID uint32, chainID ids.ID) *Rules {
	return &Rules{g, networkID, chainID}
}

func (*Rules) GetWarpConfig(ids.ID) (bool, uint64, uint64) {
	return false, 0, 0
}

func (r *Rules) NetworkID() uint32 {
	return r.networkID
}

func (r *Rules) ChainID() ids.ID {
	return r.chainID
}

func (r *Rules) GetMinBlockGap() int64 {
	return r.g.MinBlockGap
}

func (r *Rules) GetWarpBaseUnits() uint64 {
	return r.g.WarpBaseUnits
}

func (r *Rules) GetWarpUnitsPerSigner() uint64 {
	return r.g.WarpUnitsPerSigner
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

func (*Rules) FetchCustom(string) (any, bool) {
	return nil, false
}

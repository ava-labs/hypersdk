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

func (r *Rules) GetMinEmptyBlockGap() int64 {
	return r.g.MinEmptyBlockGap
}

func (r *Rules) GetValidityWindow() int64 {
	return r.g.ValidityWindow
}

func (r *Rules) GetMaxBlockUnits() chain.Dimensions {
	return r.g.MaxBlockUnits
}

func (r *Rules) GetBaseComputeUnits() uint64 {
	return r.g.BaseComputeUnits
}

func (r *Rules) GetBaseWarpComputeUnits() uint64 {
	return r.g.BaseWarpComputeUnits
}

func (r *Rules) GetWarpComputeUnitsPerSigner() uint64 {
	return r.g.WarpComputeUnitsPerSigner
}

func (r *Rules) GetOutgoingWarpComputeUnits() uint64 {
	return r.g.OutgoingWarpComputeUnits
}

func (r *Rules) GetColdStorageKeyReadUnits() uint64 {
	return r.g.ColdStorageKeyReadUnits
}

func (r *Rules) GetColdStorageValueReadUnits() uint64 {
	return r.g.ColdStorageValueReadUnits
}

func (r *Rules) GetWarmStorageKeyReadUnits() uint64 {
	return r.g.WarmStorageKeyReadUnits
}

func (r *Rules) GetWarmStorageValueReadUnits() uint64 {
	return r.g.WarmStorageValueReadUnits
}

func (r *Rules) GetStorageKeyCreateUnits() uint64 {
	return r.g.StorageKeyCreateUnits
}

func (r *Rules) GetStorageValueCreateUnits() uint64 {
	return r.g.StorageValueCreateUnits
}

func (r *Rules) GetColdStorageKeyModificationUnits() uint64 {
	return r.g.ColdStorageKeyModificationUnits
}

func (r *Rules) GetColdStorageValueModificationUnits() uint64 {
	return r.g.ColdStorageValueModificationUnits
}

func (r *Rules) GetWarmStorageKeyModificationUnits() uint64 {
	return r.g.WarmStorageKeyModificationUnits
}

func (r *Rules) GetWarmStorageValueModificationUnits() uint64 {
	return r.g.WarmStorageValueModificationUnits
}

func (r *Rules) GetMinUnitPrice() chain.Dimensions {
	return r.g.MinUnitPrice
}

func (r *Rules) GetUnitPriceChangeDenominator() chain.Dimensions {
	return r.g.UnitPriceChangeDenominator
}

func (r *Rules) GetWindowTargetUnits() chain.Dimensions {
	return r.g.WindowTargetUnits
}

func (*Rules) FetchCustom(string) (any, bool) {
	return nil, false
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
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

func (*Rules) GetSponsorStateKeysMaxChunks() []uint16 {
	return []uint16{storage.BalanceChunks}
}

func (r *Rules) GetStorageKeyReadUnits() uint64 {
	return r.g.StorageKeyReadUnits
}

func (r *Rules) GetStorageValueReadUnits() uint64 {
	return r.g.StorageValueReadUnits
}

func (r *Rules) GetStorageKeyAllocateUnits() uint64 {
	return r.g.StorageKeyAllocateUnits
}

func (r *Rules) GetStorageValueAllocateUnits() uint64 {
	return r.g.StorageValueAllocateUnits
}

func (r *Rules) GetStorageKeyWriteUnits() uint64 {
	return r.g.StorageKeyWriteUnits
}

func (r *Rules) GetStorageValueWriteUnits() uint64 {
	return r.g.StorageValueWriteUnits
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

func (r *Rules) GetBlockExecutionDepth() uint64 {
	return 5
}

func (r *Rules) GetChunksPerBlock() int {
	return 16
}

func (r *Rules) GetEpochDuration() int64 {
	// TODO: ensure this is greater than validity window
	return 15000 // 15s
}

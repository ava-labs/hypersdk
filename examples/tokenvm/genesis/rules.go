// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/fees"
)

var (
	_ chain.Rules   = (*Rules)(nil)
	_ fees.Metadata = (*FeeMetadata)(nil)
)

type Rules struct {
	g *Genesis

	networkID uint32
	chainID   ids.ID
}

type FeeMetadata struct {
	*Rules
}

// TODO: use upgradeBytes
func (g *Genesis) Rules(_ int64, networkID uint32, chainID ids.ID) *Rules {
	return &Rules{g, networkID, chainID}
}

func (*Rules) GetWarpConfig(ids.ID) (bool, uint64, uint64) {
	// We allow inbound transfers from all sources as long as 80% of stake has
	// signed a message.
	//
	// This is safe because the tokenvm scopes all assets by their source chain.
	return true, 4, 5
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

func (r *Rules) Fees() fees.Metadata {
	return &FeeMetadata{r}
}

func (f *FeeMetadata) GetMinUnitPrice() fees.Dimensions {
	return f.Rules.g.MinUnitPrice
}

func (f *FeeMetadata) GetUnitPriceChangeDenominator() fees.Dimensions {
	return f.Rules.g.UnitPriceChangeDenominator
}

func (f *FeeMetadata) GetWindowTargetUnits() fees.Dimensions {
	return f.Rules.g.WindowTargetUnits
}

func (f *FeeMetadata) GetMaxBlockUnits() fees.Dimensions {
	return f.Rules.g.MaxBlockUnits
}

func (*Rules) FetchCustom(string) (any, bool) {
	return nil, false
}

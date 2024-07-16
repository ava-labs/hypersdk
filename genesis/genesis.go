// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/fees"
)

const defaultSponsorStateKeysMaxChunks = 1

type CustomAllocation struct {
	Address string `json:"address"` // bech32 address
	Balance uint64 `json:"balance"`
}

type Genesis struct {
	// State Parameters
	StateBranchFactor merkledb.BranchFactor `json:"stateBranchFactor"`

	// Chain Parameters
	MinBlockGap      int64 `json:"minBlockGap"`      // ms
	MinEmptyBlockGap int64 `json:"minEmptyBlockGap"` // ms

	// Chain Fee Parameters
	MinUnitPrice               fees.Dimensions `json:"minUnitPrice"`
	UnitPriceChangeDenominator fees.Dimensions `json:"unitPriceChangeDenominator"`
	WindowTargetUnits          fees.Dimensions `json:"windowTargetUnits"` // 10s
	MaxBlockUnits              fees.Dimensions `json:"maxBlockUnits"`     // must be possible to reach before block too large

	// Tx Parameters
	ValidityWindow      int64 `json:"validityWindow"` // ms
	MaxActionsPerTx     uint8 `json:"maxActionsPerTx"`
	MaxOutputsPerAction uint8 `json:"maxOutputsPerAction"`

	// Tx Fee Parameters
	BaseComputeUnits          uint64 `json:"baseUnits"`
	StorageKeyReadUnits       uint64 `json:"storageKeyReadUnits"`
	StorageValueReadUnits     uint64 `json:"storageValueReadUnits"` // per chunk
	StorageKeyAllocateUnits   uint64 `json:"storageKeyAllocateUnits"`
	StorageValueAllocateUnits uint64 `json:"storageValueAllocateUnits"` // per chunk
	StorageKeyWriteUnits      uint64 `json:"storageKeyWriteUnits"`
	StorageValueWriteUnits    uint64 `json:"storageValueWriteUnits"` // per chunk

	// Allocates
	CustomAllocation []*CustomAllocation `json:"customAllocation"`
}

func Default() Genesis {
	return Genesis{
		// State Parameters
		StateBranchFactor: merkledb.BranchFactor16,

		// Chain Parameters
		MinBlockGap:      100,
		MinEmptyBlockGap: 2_500,

		// Chain Fee Parameters
		MinUnitPrice:               fees.Dimensions{100, 100, 100, 100, 100},
		UnitPriceChangeDenominator: fees.Dimensions{48, 48, 48, 48, 48},
		WindowTargetUnits:          fees.Dimensions{20_000_000, 1_000, 1_000, 1_000, 1_000},
		MaxBlockUnits:              fees.Dimensions{1_800_000, 2_000, 2_000, 2_000, 2_000},

		// Tx Parameters
		ValidityWindow:      60_000, // 60s
		MaxActionsPerTx:     16,
		MaxOutputsPerAction: 1,

		// Tx Fee Compute Parameters
		BaseComputeUnits: 1,

		// Tx Fee Storage Parameters
		//
		// TODO: tune this
		StorageKeyReadUnits:       5,
		StorageValueReadUnits:     2,
		StorageKeyAllocateUnits:   20,
		StorageValueAllocateUnits: 5,
		StorageKeyWriteUnits:      10,
		StorageValueWriteUnits:    3,
	}
}

func (g *Genesis) GetStateBranchFactor() merkledb.BranchFactor {
	return g.StateBranchFactor
}

func (g *Genesis) GetMinBlockGap() int64 {
	return g.MinBlockGap
}

func (g *Genesis) GetMinEmptyBlockGap() int64 {
	return g.MinEmptyBlockGap
}

func (g *Genesis) GetValidityWindow() int64 {
	return g.ValidityWindow
}

func (g *Genesis) GetMaxActionsPerTx() uint8 {
	return g.MaxActionsPerTx
}

func (g *Genesis) GetMaxOutputsPerAction() uint8 {
	return g.MaxOutputsPerAction
}

func (g *Genesis) GetMaxBlockUnits() fees.Dimensions {
	return g.MaxBlockUnits
}

func (g *Genesis) GetBaseComputeUnits() uint64 {
	return g.BaseComputeUnits
}

func (*Genesis) GetSponsorStateKeysMaxChunks() []uint16 {
	return []uint16{defaultSponsorStateKeysMaxChunks}
}

func (g *Genesis) GetStorageKeyReadUnits() uint64 {
	return g.StorageKeyReadUnits
}

func (g *Genesis) GetStorageValueReadUnits() uint64 {
	return g.StorageValueReadUnits
}

func (g *Genesis) GetStorageKeyAllocateUnits() uint64 {
	return g.StorageKeyAllocateUnits
}

func (g *Genesis) GetStorageValueAllocateUnits() uint64 {
	return g.StorageValueAllocateUnits
}

func (g *Genesis) GetStorageKeyWriteUnits() uint64 {
	return g.StorageKeyWriteUnits
}

func (g *Genesis) GetStorageValueWriteUnits() uint64 {
	return g.StorageValueWriteUnits
}

func (g *Genesis) GetMinUnitPrice() fees.Dimensions {
	return g.MinUnitPrice
}

func (g *Genesis) GetUnitPriceChangeDenominator() fees.Dimensions {
	return g.UnitPriceChangeDenominator
}

func (g *Genesis) GetWindowTargetUnits() fees.Dimensions {
	return g.WindowTargetUnits
}

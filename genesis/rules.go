// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/fees"

	hconsts "github.com/ava-labs/hypersdk/consts"
)

var (
	_ chain.Rules       = (*Rules)(nil)
	_ chain.RuleFactory = (*ImmutableRuleFactory)(nil)
)

type Rules struct {
	// TODO: NetworkID and ChainID are populated via Load, not by parsing the genesis.
	// Make this explicit by making these private fields and/or not including them in the JSON output.
	NetworkID uint32 `json:"networkID"`
	ChainID   ids.ID `json:"chainID"`

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
	BaseComputeUnits          uint64   `json:"baseUnits"`
	StorageKeyReadUnits       uint64   `json:"storageKeyReadUnits"`
	StorageValueReadUnits     uint64   `json:"storageValueReadUnits"` // per chunk
	StorageKeyAllocateUnits   uint64   `json:"storageKeyAllocateUnits"`
	StorageValueAllocateUnits uint64   `json:"storageValueAllocateUnits"` // per chunk
	StorageKeyWriteUnits      uint64   `json:"storageKeyWriteUnits"`
	StorageValueWriteUnits    uint64   `json:"storageValueWriteUnits"` // per chunk
	SponsorStateKeysMaxChunks []uint16 `json:"sponsorStateKeysMaxChunks"`
}

func NewDefaultRules() *Rules {
	return &Rules{
		// Chain Parameters
		MinBlockGap:      100,
		MinEmptyBlockGap: 750,

		// Chain Fee Parameters
		MinUnitPrice:               fees.Dimensions{100, 100, 100, 100, 100},
		UnitPriceChangeDenominator: fees.Dimensions{48, 48, 48, 48, 48},
		WindowTargetUnits:          fees.Dimensions{20_000_000, 1_000, 1_000, 1_000, 1_000},
		MaxBlockUnits:              fees.Dimensions{1_800_000, 2_000, 2_000, 2_000, 2_000},

		// Tx Parameters
		ValidityWindow:      60 * hconsts.MillisecondsPerSecond, // ms
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
		SponsorStateKeysMaxChunks: []uint16{1},
	}
}

func (r *Rules) GetNetworkID() uint32 { return r.NetworkID }

func (r *Rules) GetChainID() ids.ID { return r.ChainID }

func (r *Rules) GetMinBlockGap() int64 {
	return r.MinBlockGap
}

func (r *Rules) GetMinEmptyBlockGap() int64 {
	return r.MinEmptyBlockGap
}

func (r *Rules) GetValidityWindow() int64 {
	return r.ValidityWindow
}

func (r *Rules) GetMaxActionsPerTx() uint8 {
	return r.MaxActionsPerTx
}

func (r *Rules) GetMaxBlockUnits() fees.Dimensions {
	return r.MaxBlockUnits
}

func (r *Rules) GetBaseComputeUnits() uint64 {
	return r.BaseComputeUnits
}

func (r *Rules) GetSponsorStateKeysMaxChunks() []uint16 {
	return r.SponsorStateKeysMaxChunks
}

func (r *Rules) GetStorageKeyReadUnits() uint64 {
	return r.StorageKeyReadUnits
}

func (r *Rules) GetStorageValueReadUnits() uint64 {
	return r.StorageValueReadUnits
}

func (r *Rules) GetStorageKeyAllocateUnits() uint64 {
	return r.StorageKeyAllocateUnits
}

func (r *Rules) GetStorageValueAllocateUnits() uint64 {
	return r.StorageValueAllocateUnits
}

func (r *Rules) GetStorageKeyWriteUnits() uint64 {
	return r.StorageKeyWriteUnits
}

func (r *Rules) GetStorageValueWriteUnits() uint64 {
	return r.StorageValueWriteUnits
}

func (r *Rules) GetMinUnitPrice() fees.Dimensions {
	return r.MinUnitPrice
}

func (r *Rules) GetUnitPriceChangeDenominator() fees.Dimensions {
	return r.UnitPriceChangeDenominator
}

func (r *Rules) GetWindowTargetUnits() fees.Dimensions {
	return r.WindowTargetUnits
}

func (*Rules) FetchCustom(string) (any, bool) {
	return nil, false
}

type ImmutableRuleFactory struct {
	Rules chain.Rules
}

func (i *ImmutableRuleFactory) GetRules(_ int64) chain.Rules {
	return i.Rules
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"encoding/json"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/fees"

	hconsts "github.com/ava-labs/hypersdk/consts"
)

var (
	_ chain.Rules              = (*BaseRules)(nil)
	_ RuleFactory[chain.Rules] = (*BaseRules)(nil)
)

type BaseRules struct {
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

// GetRules would normally depend on the param, but these rules are unchanging
func (r *BaseRules) GetRules(_ int64) chain.Rules {
	return r
}

func DefaultRules() *BaseRules {
	return &BaseRules{
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

func LoadBaseRules(b []byte, _ []byte /* upgradeBytes */) (*BaseRules, error) {
	g := DefaultRules()
	if len(b) > 0 {
		if err := json.Unmarshal(b, g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal base genesis %s: %w", string(b), err)
		}
	}
	return g, nil
}

func (r *BaseRules) GetNetworkID() uint32 {
	return r.NetworkID
}

func (r *BaseRules) GetChainID() ids.ID {
	return r.ChainID
}

func (r *BaseRules) GetMinBlockGap() int64 {
	return r.MinBlockGap
}

func (r *BaseRules) GetMinEmptyBlockGap() int64 {
	return r.MinEmptyBlockGap
}

func (r *BaseRules) GetValidityWindow() int64 {
	return r.ValidityWindow
}

func (r *BaseRules) GetMaxActionsPerTx() uint8 {
	return r.MaxActionsPerTx
}

func (r *BaseRules) GetMaxOutputsPerAction() uint8 {
	return r.MaxOutputsPerAction
}

func (r *BaseRules) GetMaxBlockUnits() fees.Dimensions {
	return r.MaxBlockUnits
}

func (r *BaseRules) GetBaseComputeUnits() uint64 {
	return r.BaseComputeUnits
}

func (r *BaseRules) GetSponsorStateKeysMaxChunks() []uint16 {
	return r.SponsorStateKeysMaxChunks
}

func (r *BaseRules) GetStorageKeyReadUnits() uint64 {
	return r.StorageKeyReadUnits
}

func (r *BaseRules) GetStorageValueReadUnits() uint64 {
	return r.StorageValueReadUnits
}

func (r *BaseRules) GetStorageKeyAllocateUnits() uint64 {
	return r.StorageKeyAllocateUnits
}

func (r *BaseRules) GetStorageValueAllocateUnits() uint64 {
	return r.StorageValueAllocateUnits
}

func (r *BaseRules) GetStorageKeyWriteUnits() uint64 {
	return r.StorageKeyWriteUnits
}

func (r *BaseRules) GetStorageValueWriteUnits() uint64 {
	return r.StorageValueWriteUnits
}

func (r *BaseRules) GetMinUnitPrice() fees.Dimensions {
	return r.MinUnitPrice
}

func (r *BaseRules) GetUnitPriceChangeDenominator() fees.Dimensions {
	return r.UnitPriceChangeDenominator
}

func (r *BaseRules) GetWindowTargetUnits() fees.Dimensions {
	return r.WindowTargetUnits
}

func (*BaseRules) FetchCustom(string) (any, bool) {
	return nil, false
}

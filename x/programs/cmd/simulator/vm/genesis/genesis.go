// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/x/merkledb"

	hconsts "github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/vm"

	"github.com/ava-labs/hypersdk/x/programs/cmd/simulator/vm/consts"
)

var _ vm.Genesis = (*Genesis)(nil)

type CustomAllocation struct {
	Address string `json:"address"` // bech32 address
	Balance uint64 `json:"balance"`
}

type Genesis struct {
	HRP string `json:"hrp"`

	// State Parameters
	StateBranchFactor         merkledb.BranchFactor `json:"stateBranchFactor"`
	SponsorStateKeysMaxChunks []uint16

	// Chain Parameters
	MinBlockGap      int64 `json:"minBlockGap"`      // ms
	MinEmptyBlockGap int64 `json:"minEmptyBlockGap"` // ms

	// Chain Fee Parameters
	MinUnitPrice               fees.Dimensions `json:"minUnitPrice"`
	UnitPriceChangeDenominator fees.Dimensions `json:"unitPriceChangeDenominator"`
	WindowTargetUnits          fees.Dimensions `json:"windowTargetUnits"` // 10s
	MaxBlockUnits              fees.Dimensions `json:"maxBlockUnits"`     // must be possible to reach before block too large

	// Tx Parameters
	ValidityWindow int64 `json:"validityWindow"` // ms

	// Tx Fee Parameters
	BaseComputeUnits          uint64 `json:"baseUnits"`
	StorageKeyReadUnits       uint64 `json:"storageKeyReadUnits"`
	StorageValueReadUnits     uint64 `json:"storageValueReadUnits"`
	StorageKeyAllocateUnits   uint64 `json:"storageKeyAllocateUnits"`
	StorageValueAllocateUnits uint64 `json:"storageValueAllocateUnits"`
	StorageKeyWriteUnits      uint64 `json:"storageKeyWriteUnits"`
	StorageValueWriteUnits    uint64 `json:"storageValueWriteUnits"`

	// program Runtime Parameters
	EnableDebugMode  bool `json:"enableDebugMode"`
	EnableBulkMemory bool `json:"enableBulkMemory"`

	// Allocates
	CustomAllocation []*CustomAllocation `json:"customAllocation"`
}

func Default() *Genesis {
	return &Genesis{
		HRP: consts.HRP,

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

		// Tx Parameters
		ValidityWindow: 60 * hconsts.MillisecondsPerSecond, // ms

		// program Runtime Parameters
		EnableDebugMode: true,
		// Enabled to only enable Wasi for testing mode
		EnableBulkMemory: true,
	}
}

func New(b []byte, _ []byte /* upgradeBytes */) (*Genesis, error) {
	g := Default()
	if len(b) > 0 {
		if err := json.Unmarshal(b, g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config %s: %w", string(b), err)
		}
	}
	return g, nil
}

func (g *Genesis) Load(ctx context.Context, tracer trace.Tracer, mu state.Mutable) error {
	if consts.HRP != g.HRP {
		return ErrInvalidHRP
	}
	if err := g.StateBranchFactor.Valid(); err != nil {
		return err
	}

	// TODO: custom allocates for more realistic fee simulation
	//
	// supply := uint64(0)
	// for _, alloc := range g.CustomAllocation {
	// 	pk, err := utils.ParseAddress(alloc.Address)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	supply, err = smath.Add64(supply, alloc.Balance)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if err := storage.SetBalance(ctx, mu, pk, alloc.Balance); err != nil {
	// 		return fmt.Errorf("%w: addr=%s, bal=%d", err, alloc.Address, alloc.Balance)
	// 	}
	// }
	return nil
}

func (g *Genesis) GetStateBranchFactor() merkledb.BranchFactor {
	return g.StateBranchFactor
}

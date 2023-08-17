// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ava-labs/avalanchego/trace"
	smath "github.com/ava-labs/avalanchego/utils/math"

	"github.com/ava-labs/hypersdk/chain"
	hconsts "github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/utils"
	"github.com/ava-labs/hypersdk/vm"
)

var _ vm.Genesis = (*Genesis)(nil)

type CustomAllocation struct {
	Address string `json:"address"` // bech32 address
	Balance uint64 `json:"balance"`
}

type Genesis struct {
	// Address prefix
	HRP string `json:"hrp"`

	// Chain Parameters
	MinBlockGap      int64 `json:"minBlockGap"`      // ms
	MinEmptyBlockGap int64 `json:"minEmptyBlockGap"` // ms

	// Chain Fee Parameters
	MinUnitPrice               chain.Dimensions `json:"minUnitPrice"`
	UnitPriceChangeDenominator chain.Dimensions `json:"unitPriceChangeDenominator"`
	WindowTargetUnits          chain.Dimensions `json:"windowTargetUnits"` // 10s
	MaxBlockUnits              chain.Dimensions `json:"maxBlockUnits"`     // must be possible to reach before block too large

	// Tx Parameters
	ValidityWindow int64 `json:"validityWindow"` // ms

	// Tx Fee Parameters
	BaseComputeUnits             uint64 `json:"baseUnits"`
	BaseWarpComputeUnits         uint64 `json:"baseWarpUnits"`
	WarpComputeUnitsPerSigner    uint64 `json:"warpUnitsPerSigner"`
	OutgoingWarpComputeUnits     uint64 `json:"outgoingWarpComputeUnits"`
	ColdStorageReadUnits         uint64 `json:"coldStorageReadUnits"`
	WarmStorageReadUnits         uint64 `json:"warmStorageReadUnits"`
	ColdStorageModificationUnits uint64 `json:"coldStorageModificationUnits"`
	WarmStorageModificationUnits uint64 `json:"warmStorageModificationUnits"`

	// Allocations
	CustomAllocation []*CustomAllocation `json:"customAllocation"`
}

func Default() *Genesis {
	return &Genesis{
		HRP: consts.HRP,

		// Chain Parameters
		MinBlockGap:      100,
		MinEmptyBlockGap: 2_500,

		// Chain Fee Parameters
		MinUnitPrice:               chain.Dimensions{100, 100, 100, 100, 100},
		UnitPriceChangeDenominator: chain.Dimensions{48, 48, 48, 48, 48},
		WindowTargetUnits:          chain.Dimensions{20_000_000, 1_000, 1_000, 1_000, 1_000},
		MaxBlockUnits:              chain.Dimensions{1_800_000, 2_000, 2_000, 2_000, 2_000},

		// Tx Parameters
		ValidityWindow: 60 * hconsts.MillisecondsPerSecond, // ms

		// Tx Fee Parameters
		BaseComputeUnits:             1,
		BaseWarpComputeUnits:         1_024,
		WarpComputeUnitsPerSigner:    128,
		OutgoingWarpComputeUnits:     1_024,
		ColdStorageReadUnits:         5,
		WarmStorageReadUnits:         1,
		ColdStorageModificationUnits: 10,
		WarmStorageModificationUnits: 1,
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

func (g *Genesis) Load(ctx context.Context, tracer trace.Tracer, db chain.Database) error {
	ctx, span := tracer.Start(ctx, "Genesis.Load")
	defer span.End()

	if consts.HRP != g.HRP {
		return ErrInvalidHRP
	}

	supply := uint64(0)
	for _, alloc := range g.CustomAllocation {
		pk, err := utils.ParseAddress(alloc.Address)
		if err != nil {
			return err
		}
		supply, err = smath.Add64(supply, alloc.Balance)
		if err != nil {
			return err
		}
		if err := storage.SetBalance(ctx, db, pk, alloc.Balance); err != nil {
			return fmt.Errorf("%w: addr=%s, bal=%d", err, alloc.Address, alloc.Balance)
		}
	}
	return nil
}

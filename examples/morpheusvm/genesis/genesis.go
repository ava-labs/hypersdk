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
	MinUnitPrice               uint64 `json:"minUnitPrice"`
	UnitPriceChangeDenominator uint64 `json:"unitPriceChangeDenominator"`
	WindowTargetUnits          uint64 `json:"windowTargetUnits"` // 10s
	MaxBlockUnits              uint64 `json:"maxBlockUnits"`     // must be possible to reach before block too large

	// Tx Parameters
	ValidityWindow int64 `json:"validityWindow"` // ms

	// Tx Fee Parameters
	BaseUnits          uint64 `json:"baseUnits"`
	WarpBaseUnits      uint64 `json:"warpBaseUnits"`
	WarpUnitsPerSigner uint64 `json:"warpUnitsPerSigner"`

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
		MinUnitPrice:               1,
		UnitPriceChangeDenominator: 48,
		WindowTargetUnits:          20_000_000,
		MaxBlockUnits:              1_800_000, // 1.8 MiB

		// Tx Parameters
		ValidityWindow: 60 * hconsts.MillisecondsPerSecond, // ms

		// Tx Fee Parameters
		BaseUnits:          48, // timestamp(8) + chainID(32) + unitPrice(8)
		WarpBaseUnits:      1_024,
		WarpUnitsPerSigner: 128,
	}
}

func New(b []byte, _ []byte /* upgradeBytes */) (*Genesis, error) {
	g := Default()
	if len(b) > 0 {
		if err := json.Unmarshal(b, g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config %s: %w", string(b), err)
		}
	}
	if g.WindowTargetUnits == 0 {
		return nil, ErrInvalidTarget
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

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ava-labs/avalanchego/trace"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/vm"

	smath "github.com/ava-labs/avalanchego/utils/math"
)

var (
	_ vm.Genesis                    = (*Genesis)(nil)
	_ vm.RuleFactory[*vm.BaseRules] = (*RuleFactory)(nil)
)

type RuleFactory struct {
	unchangingRules *vm.BaseRules
}

func (r *RuleFactory) GetRules(_ int64) *vm.BaseRules {
	return r.unchangingRules
}

type CustomAllocation struct {
	Address string `json:"address"` // bech32 address
	Balance uint64 `json:"balance"`
}

type Genesis struct {
	*vm.BaseRules

	// Allocates
	CustomAllocation []*CustomAllocation `json:"customAllocation"`
}

func (g *Genesis) GetRulesFactory() vm.RuleFactory[*vm.BaseRules] {
	return &RuleFactory{g.BaseRules}
}

func Default() *Genesis {
	return &Genesis{
		BaseRules: vm.DefaultRules(),
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
	ctx, span := tracer.Start(ctx, "Genesis.Load")
	defer span.End()

	supply := uint64(0)
	for _, alloc := range g.CustomAllocation {
		addr, err := codec.ParseAddressBech32(consts.HRP, alloc.Address)
		if err != nil {
			return fmt.Errorf("%w: %s", err, alloc.Address)
		}
		supply, err = smath.Add64(supply, alloc.Balance)
		if err != nil {
			return err
		}
		if err := storage.SetBalance(ctx, mu, addr, alloc.Balance); err != nil {
			return fmt.Errorf("%w: addr=%s, bal=%d", err, alloc.Address, alloc.Balance)
		}
	}
	return nil
}

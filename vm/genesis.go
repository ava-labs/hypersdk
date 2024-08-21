// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"

	safemath "github.com/ava-labs/avalanchego/utils/math"
)

var _ Genesis = (*BaseGenesis)(nil)

type CustomAllocation struct {
	Address string `json:"address"`
	Balance uint64 `json:"balance"`
}

type BaseGenesis struct {
	StateBranchFactor merkledb.BranchFactor `json:"stateBranchFactor"`

	// Allocates
	CustomAllocation []*CustomAllocation `json:"customAllocation"`
}

func (g *BaseGenesis) GetStateBranchFactor() merkledb.BranchFactor {
	return g.StateBranchFactor
}

func DefaultGenesis() *BaseGenesis {
	return &BaseGenesis{
		StateBranchFactor: merkledb.BranchFactor16,
		CustomAllocation:  []*CustomAllocation{},
	}
}

func (g *BaseGenesis) InitializeState(ctx context.Context, tracer trace.Tracer, mu state.Mutable, am AllocationManager) error {
	ctx, span := tracer.Start(ctx, "Genesis.Load")
	defer span.End()

	supply := uint64(0)
	for _, alloc := range g.CustomAllocation {
		addr, err := codec.ParseAnyHrpAddressBech32(alloc.Address)
		if err != nil {
			return fmt.Errorf("%w: %s", err, alloc.Address)
		}
		supply, err = safemath.Add[uint64](supply, alloc.Balance)
		if err != nil {
			return err
		}
		if err := am.SetBalance(ctx, mu, addr, alloc.Balance); err != nil {
			return fmt.Errorf("%w: addr=%s, bal=%d", err, alloc.Address, alloc.Balance)
		}
	}
	return nil
}

func LoadBaseGenesis(b []byte) (Genesis, error) {
	g := DefaultGenesis()
	if len(b) > 0 {
		if err := json.Unmarshal(b, g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config %s: %w", string(b), err)
		}
	}
	return g, nil
}

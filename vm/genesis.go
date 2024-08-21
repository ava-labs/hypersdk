// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	safemath "github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/status-im/keycard-go/hexutils"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/state"
)

var _ Genesis = (*BaseGenesis)(nil)

type BaseGenesis struct {
	StateBranchFactor merkledb.BranchFactor `json:"stateBranchFactor"`

	InitialState map[string]string `json:"initialState"`
}

func (g *BaseGenesis) GetStateBranchFactor() merkledb.BranchFactor {
	return g.StateBranchFactor
}

func DefaultGenesis() *BaseGenesis {
	return &BaseGenesis{
		StateBranchFactor: merkledb.BranchFactor16,
		InitialState:      map[string]string{},
	}
}

func (g *BaseGenesis) InitializeState(ctx context.Context, tracer trace.Tracer, mu state.Mutable) error {
	ctx, span := tracer.Start(ctx, "Genesis.Load")
	defer span.End()

	for key, value := range g.InitialState {
		if err := mu.Insert(ctx, hexutils.HexToBytes(key), hexutils.HexToBytes(value)); err != nil {
			return err
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

type Bech32Allocation struct {
	Address string `json:"address"`
	Balance uint64 `json:"balance"`
}

type Bech32AllocationGenesis struct {
	BaseGenesis
	manager          AllocationManager
	CustomAllocation []*Bech32Allocation `json:"customAllocation"`
}

func DefaultBech32AllocationGenesis(manager AllocationManager) *Bech32AllocationGenesis {
	return &Bech32AllocationGenesis{
		manager: manager,
		BaseGenesis: BaseGenesis{
			StateBranchFactor: merkledb.BranchFactor16,
			InitialState:      map[string]string{},
		},
	}
}

func (g *Bech32AllocationGenesis) InitializeState(ctx context.Context, tracer trace.Tracer, mu state.Mutable) error {
	ctx, span := tracer.Start(ctx, "Genesis.Load")
	defer span.End()

	if err := g.BaseGenesis.InitializeState(ctx, tracer, mu); err != nil {
		return nil
	}

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
		if err := g.manager.SetBalance(ctx, mu, addr, alloc.Balance); err != nil {
			return fmt.Errorf("%w: addr=%s, bal=%d", err, alloc.Address, alloc.Balance)
		}
	}

	return nil
}

func LoadBech32AllocationGenesis(b []byte, manager AllocationManager) (Genesis, error) {
	g := DefaultBech32AllocationGenesis(manager)
	if len(b) > 0 {
		if err := json.Unmarshal(b, g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config %s: %w", string(b), err)
		}
	}
	return g, nil
}

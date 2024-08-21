// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/json"
	"fmt"
	safemath "github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/hypersdk/codec"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/state"
)

var _ Genesis = (*StateBranchFactorGenesis)(nil)

type StateBranchFactorGenesis struct {
	StateBranchFactor merkledb.BranchFactor `json:"stateBranchFactor"`
}

func (g *StateBranchFactorGenesis) GetStateBranchFactor() merkledb.BranchFactor {
	return g.StateBranchFactor
}

func NewStateBranchFactorGenesis() *StateBranchFactorGenesis {
	return &StateBranchFactorGenesis{
		StateBranchFactor: merkledb.BranchFactor16,
	}
}

func (g *StateBranchFactorGenesis) InitializeState(ctx context.Context, tracer trace.Tracer, _ state.Mutable) error {
	ctx, span := tracer.Start(ctx, "Genesis.Load")
	defer span.End()
	return nil
}

func LoadStateBranchFactorGenesis(b []byte) (Genesis, error) {
	g := NewStateBranchFactorGenesis()
	if len(b) > 0 {
		if err := json.Unmarshal(b, g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config %s: %w", string(b), err)
		}
	}
	return g, nil
}

var _ Genesis = (*Bech32Genesis)(nil)

type CustomAllocation struct {
	Address string `json:"address"`
	Balance uint64 `json:"balance"`
}

type Bech32Genesis struct {
	*StateBranchFactorGenesis

	// Allocates
	CustomAllocation []*CustomAllocation `json:"customAllocation"`
	setBalance       func(ctx context.Context, mu state.Mutable, addr codec.Address, balance uint64) error
}

func NewBech32Genesis(setBalance func(ctx context.Context, mu state.Mutable, addr codec.Address, balance uint64) error) *Bech32Genesis {
	return &Bech32Genesis{
		StateBranchFactorGenesis: NewStateBranchFactorGenesis(),
		CustomAllocation:         []*CustomAllocation{},
		setBalance:               setBalance,
	}
}

func (g *Bech32Genesis) InitializeState(ctx context.Context, tracer trace.Tracer, mu state.Mutable) error {
	if err := g.StateBranchFactorGenesis.InitializeState(ctx, tracer, mu); err != nil {
		return err
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
		if err := g.setBalance(ctx, mu, addr, alloc.Balance); err != nil {
			return fmt.Errorf("%w: addr=%s, bal=%d", err, alloc.Address, alloc.Balance)
		}
	}
	return nil
}

func LoadBech32Genesis(b []byte, setBalance func(ctx context.Context, mu state.Mutable, addr codec.Address, balance uint64) error) (*Bech32Genesis, error) {
	g := NewBech32Genesis(setBalance)
	if len(b) > 0 {
		if err := json.Unmarshal(b, g); err != nil {
			return nil, fmt.Errorf("failed to unmarshal config %s: %w", string(b), err)
		}
	}
	return g, nil
}

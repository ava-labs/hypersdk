// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"

	safemath "github.com/ava-labs/avalanchego/utils/math"
)

var _ Genesis = (*DefaultGenesis)(nil)

type CustomAllocation struct {
	Address string `json:"address"`
	Balance uint64 `json:"balance"`
}

type DefaultGenesis struct {
	StateBranchFactor merkledb.BranchFactor `json:"stateBranchFactor"`
	CustomAllocation  []*CustomAllocation   `json:"customAllocation"`
}

func NewDefaultGenesis(customAllocations []*CustomAllocation) *DefaultGenesis {
	return &DefaultGenesis{
		StateBranchFactor: merkledb.BranchFactor16,
		CustomAllocation:  customAllocations,
	}
}

func (g *DefaultGenesis) InitializeState(ctx context.Context, tracer trace.Tracer, mu state.Mutable, balanceHandler chain.BalanceHandler) error {
	_, span := tracer.Start(ctx, "Genesis.InitializeState")
	defer span.End()

	supply := uint64(0)
	for _, alloc := range g.CustomAllocation {
		addr, err := codec.ParseAnyHrpAddressBech32(alloc.Address) // TODO: allow VM to specify required HRP
		if err != nil {
			return fmt.Errorf("%w: %s", err, alloc.Address)
		}
		supply, err = safemath.Add[uint64](supply, alloc.Balance)
		if err != nil {
			return err
		}
		if err := balanceHandler.AddBalance(ctx, addr, mu, alloc.Balance, true); err != nil {
			return fmt.Errorf("%w: addr=%s, bal=%d", err, alloc.Address, alloc.Balance)
		}
	}
	return nil
}

func (g *DefaultGenesis) GetStateBranchFactor() merkledb.BranchFactor {
	return g.StateBranchFactor
}

type defaultGenesisAndRuleFactory struct{}

type DefaultGenesisWithRules struct {
	Genesis      DefaultGenesis `json:"genesis"`
	GenesisRules Rules          `json:"initialRules"`
}

func (defaultGenesisAndRuleFactory) Load(genesisBytes []byte, _ []byte, networkID uint32, chainID ids.ID) (Genesis, RuleFactory, error) {
	genesisWithRules := &DefaultGenesisWithRules{}
	if err := json.Unmarshal(genesisBytes, genesisWithRules); err != nil {
		return nil, nil, err
	}
	rules := genesisWithRules.GenesisRules
	rules.NetworkID = networkID
	rules.ChainID = chainID

	return &genesisWithRules.Genesis, &ImmutableRuleFactory{&rules}, nil
}

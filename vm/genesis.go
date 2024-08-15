// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"
	"fmt"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/x/merkledb"

	smath "github.com/ava-labs/avalanchego/utils/math"
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

func Default() *BaseGenesis {
	return &BaseGenesis{
		StateBranchFactor: merkledb.BranchFactor16,
		CustomAllocation:  []*CustomAllocation{},
	}
}

func (g *BaseGenesis) LoadAllocations(ctx context.Context, tracer trace.Tracer, addressParser AddressParser, am AllocationManager) error {
	ctx, span := tracer.Start(ctx, "Genesis.Load")
	defer span.End()

	supply := uint64(0)
	for _, alloc := range g.CustomAllocation {
		addr, err := addressParser.ParseAddress(alloc.Address)
		if err != nil {
			return fmt.Errorf("%w: %s", err, alloc.Address)
		}
		supply, err = smath.Add64(supply, alloc.Balance)
		if err != nil {
			return err
		}
		if err := am.SetBalance(addr, alloc.Balance); err != nil {
			return fmt.Errorf("%w: addr=%s, bal=%d", err, alloc.Address, alloc.Balance)
		}
	}
	return nil
}

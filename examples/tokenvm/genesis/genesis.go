// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/rules"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/vm"

	smath "github.com/ava-labs/avalanchego/utils/math"
)

var _ vm.Genesis = (*Genesis)(nil)

type Genesis struct {
	rules.Genesis
}

func Default() *Genesis {
	return &Genesis{
		Genesis: rules.Default(),
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

	if err := g.StateBranchFactor.Valid(); err != nil {
		return err
	}

	supply := uint64(0)
	for _, alloc := range g.CustomAllocation {
		pk, err := codec.ParseAddressBech32(consts.HRP, alloc.Address)
		if err != nil {
			return err
		}
		supply, err = smath.Add64(supply, alloc.Balance)
		if err != nil {
			return err
		}
		if err := storage.SetBalance(ctx, mu, pk, ids.Empty, alloc.Balance); err != nil {
			return fmt.Errorf("%w: addr=%s, bal=%d", err, alloc.Address, alloc.Balance)
		}
	}
	return storage.SetAsset(
		ctx,
		mu,
		ids.Empty,
		[]byte(consts.Symbol),
		consts.Decimals,
		[]byte(consts.Name),
		supply,
		codec.EmptyAddress,
	)
}

func (g *Genesis) GetStateBranchFactor() merkledb.BranchFactor {
	return g.StateBranchFactor
}

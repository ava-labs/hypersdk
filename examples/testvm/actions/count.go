// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/testvm/consts"
	"github.com/ava-labs/hypersdk/examples/testvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

const (
	CountComputeUnits = 1
)

var (
	ErrOutputValueZero                 = errors.New("value is zero")
	ErrOutputMemoTooLarge              = errors.New("memo is too large")
	_                     chain.Action = (*Count)(nil)
)

type Count struct {
	// Amount to increment actor.
	Amount uint64 `serialize:"true" json:"value"`
}

func (*Count) GetTypeID() uint8 {
	return consts.CountID
}

func (c *Count) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.CounterKey(actor)): state.All,
	}
}

func (c *Count) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) (codec.Typed, error) {
	err := storage.IncreaseCounter(ctx, mu, actor, c.Amount)
	if err != nil {
		return nil, err
	}

	count, err := storage.GetCounter(ctx, mu, actor)
	if err != nil {
		return nil, err
	}

	return &CountResult{
		ActorCount: count,
	}, nil
}

func (*Count) ComputeUnits(chain.Rules) uint64 {
	return CountComputeUnits
}

func (*Count) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

var _ codec.Typed = (*CountResult)(nil)

type CountResult struct {
	ActorCount uint64 `serialize:"true" json:"actor_count"`
}

func (*CountResult) GetTypeID() uint8 {
	return consts.CountID // Common practice is to use the action ID
}

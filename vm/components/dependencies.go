package components

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

type ActionComponent interface {
	ComputeUnits(chain.Rules) uint64

	StateKeys(actor codec.Address, actionID ids.ID) state.Keys

	Execute(
		ctx context.Context,
		r chain.Rules,
		mu state.Mutable,
		timestamp int64,
		actor codec.Address,
		actionID ids.ID,
	) (interface{}, error)
}
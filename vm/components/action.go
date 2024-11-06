package components

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.Action = &GenericAction{}


// wrapper over a generic action that doesn't have a type ID
type GenericAction struct {
	// Action is the underlying action
	Action ActionComponent

	// type ID of the action
	TypeID uint8

	// range
	ValidRangeStart int64
	ValidRangeEnd   int64
}

func NewGenericAction(action ActionComponent, typeID uint8, start int64, end int64) *GenericAction {
	return &GenericAction{
		Action: action,
		TypeID: typeID,
	}
}

func (g GenericAction) ComputeUnits(rules chain.Rules) uint64 {
	return g.Action.ComputeUnits(rules)
}

func (g *GenericAction) GetTypeID() uint8 {
	return g.TypeID
}

func (g *GenericAction) StateKeys(actor codec.Address, actionID ids.ID) state.Keys {
	return g.Action.StateKeys(actor, actionID)
}

func (g *GenericAction) Execute(
	ctx context.Context,
	rules chain.Rules,
	mu state.Mutable,
	timestamp int64,
	actor codec.Address,
	actionID ids.ID,
) (codec.Typed, error) {
	res, err := g.Action.Execute(ctx, rules, mu, timestamp, actor, actionID)
	if err != nil {
		return nil, err
	}

	return NewGenericType(res, g.TypeID), nil
}

func (g *GenericAction) ValidRange(rules chain.Rules) (int64, int64) {
	return g.ValidRangeStart, g.ValidRangeEnd
}

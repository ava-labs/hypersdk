// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.BalanceHandler = (*balanceHandler)(nil)

func NewTestBalanceHandler(canDeductError, deductError error) chain.BalanceHandler {
	return &balanceHandler{
		canDeductError,
		deductError,
	}
}

type balanceHandler struct {
	canDeductError error
	deductError    error
}

func (*balanceHandler) AddBalance(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	panic("unimplemented")
}

func (m *balanceHandler) CanDeduct(_ context.Context, _ codec.Address, _ state.Immutable, _ uint64) error {
	return m.canDeductError
}

func (m *balanceHandler) Deduct(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	return m.deductError
}

func (*balanceHandler) GetBalance(_ context.Context, _ codec.Address, _ state.Immutable) (uint64, error) {
	panic("unimplemented")
}

func (*balanceHandler) SponsorStateKeys(_ codec.Address) state.Keys {
	return state.Keys{}
}

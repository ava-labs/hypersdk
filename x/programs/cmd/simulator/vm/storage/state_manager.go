// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

type StateManager struct{}

func (m *StateManager) SponsorStateKeys(_ codec.Address) state.Keys {
	// TODO implement me
	panic("implement me")
}

func (m *StateManager) CanDeduct(_ context.Context, _ codec.Address, _ state.Immutable, _ uint64) error {
	// TODO implement me
	panic("implement me")
}

func (m *StateManager) Deduct(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	// TODO implement me
	panic("implement me")
}

func (m *StateManager) Refund(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	// TODO implement me
	panic("implement me")
}

func (*StateManager) HeightKey() []byte {
	return HeightKey()
}

func (*StateManager) TimestampKey() []byte {
	return TimestampKey()
}

func (*StateManager) FeeKey() []byte {
	return FeeKey()
}

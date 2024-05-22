// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"context"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

type StateManager struct{}

func (*StateManager) SponsorStateKeys(_ codec.Address) state.Keys {
	// TODO implement me
	panic("implement me")
}

func (*StateManager) CanDeduct(_ context.Context, _ codec.Address, _ state.Immutable, _ uint64) error {
	// TODO implement me
	panic("implement me")
}

func (*StateManager) Deduct(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	// TODO implement me
	panic("implement me")
}

<<<<<<< HEAD:x/programs/runtime/vm/storage/state_manager.go
=======
func (*StateManager) Refund(_ context.Context, _ codec.Address, _ state.Mutable, _ uint64) error {
	// TODO implement me
	panic("implement me")
}

>>>>>>> 32a72b9e (linting):x/programs/v2/vm/storage/state_manager.go
func (*StateManager) HeightKey() []byte {
	return HeightKey()
}

func (*StateManager) TimestampKey() []byte {
	return TimestampKey()
}

func (*StateManager) FeeKey() []byte {
	return FeeKey()
}

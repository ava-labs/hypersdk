// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/codec"
)

type Immutable interface {
	GetValue(ctx context.Context, key []byte) (value []byte, err error)
}

type Mutable interface {
	Immutable

	Insert(ctx context.Context, key []byte, value []byte) error
	Remove(ctx context.Context, key []byte) error
}

type View interface {
	Immutable

	NewView(ctx context.Context, changes merkledb.ViewChanges) (merkledb.View, error)
	GetMerkleRoot(ctx context.Context) (ids.ID, error)
}

// VM state are state keys managed by hypersdk exposed to actions
// TODO naming
type VM interface {
	GetBalance(ctx context.Context, address codec.Address) (uint64, bool, error)
	PutBalance(ctx context.Context, address codec.Address, amount uint64) (uint64, bool, error)
	RemoveBalance(ctx context.Context, address codec.Address) (uint64, error)
}

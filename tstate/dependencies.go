// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"

	"github.com/ava-labs/avalanchego/x/merkledb"
)

type Database interface {
	NewViewFromMap(ctx context.Context, changes map[string]*merkledb.ChangeOp, copyBytes bool) (merkledb.TrieView, error)
	GetValue(ctx context.Context, key []byte) (value []byte, err error)
}

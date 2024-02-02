// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"

	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/x/merkledb"
)

type Database interface {
	NewViewFromMap(ctx context.Context, changes map[string]maybe.Maybe[[]byte], copyBytes bool) (merkledb.View, error)
	GetValue(ctx context.Context, key []byte) (value []byte, err error)
}

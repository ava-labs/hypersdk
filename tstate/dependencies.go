// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import (
	"context"
)

type Database interface {
	GetValue(ctx context.Context, key []byte) (value []byte, err error)
}

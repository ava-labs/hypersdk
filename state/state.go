// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"
)

// We use string here because we often store items in a map before storage
// in a database. This allows us to avoid casting back and forth.
type Immutable interface {
	Get(ctx context.Context, key string) (value []byte, err error)
}

type Mutable interface {
	Immutable

	Put(ctx context.Context, key string, value []byte) error
	Delete(ctx context.Context, key string) error
}

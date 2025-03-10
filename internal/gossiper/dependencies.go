// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossiper

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
)

type Mempool[T any] interface {
	Top(
		ctx context.Context,
		targetDuration time.Duration,
		f func(context.Context, T) (cont bool, restore bool, err error),
	) error
}

type Serializer[T any] interface {
	Marshal([]T) []byte
	Unmarshal(b []byte) ([]T, error)
}

type Submitter[T any] interface {
	Submit(context.Context, []T) []error
}

type ValidatorSet interface {
	NodeID() ids.NodeID
	Proposers(ctx context.Context, diff int, depth int) (set.Set[ids.NodeID], error)
	IsValidator(ctx context.Context, nodeID ids.NodeID) (bool, error)
}

type Tx interface {
	GetID() ids.ID
	GetExpiry() int64
	Size() int
}

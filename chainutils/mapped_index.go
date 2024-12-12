// Copyright (C) 2024, Ava Labs, Inv. All rights reserved.
// See the file LICENSE for licensing terms.

package chainutils

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
)

type ChainIndex[I any] interface {
	GetBlock(ctx context.Context, blkID ids.ID) (I, error)
}

type mappedIndex[I any, O any] struct {
	ChainIndex ChainIndex[I]
	f          func(I) O
}

func MapIndex[I any, O any](c ChainIndex[I], f func(I) O) ChainIndex[O] {
	return &mappedIndex[I, O]{
		ChainIndex: c,
		f:          f,
	}
}

func (m *mappedIndex[I, O]) GetBlock(ctx context.Context, blkID ids.ID) (O, error) {
	blk, err := m.ChainIndex.GetBlock(ctx, blkID)
	if err != nil {
		var emptyBlk O
		return emptyBlk, err
	}
	return m.f(blk), nil
}

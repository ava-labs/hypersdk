// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindowtest

import (
	"context"

	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/internal/emap"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
)

var _ validitywindow.Interface[emap.Item] = (*MockTimeValidityWindow[emap.Item])(nil)

type MockTimeValidityWindow[T emap.Item] struct {
	OnVerifyExpiryReplayProtection func(ctx context.Context, block validitywindow.ExecutionBlock[T]) error
	OnIsRepeat                     func(ctx context.Context, parentBlk validitywindow.ExecutionBlock[T], containers []T, currentTime int64) (set.Bits, error)
}

func (*MockTimeValidityWindow[T]) Accept(validitywindow.ExecutionBlock[T]) {
}

func (m *MockTimeValidityWindow[T]) VerifyExpiryReplayProtection(ctx context.Context, blk validitywindow.ExecutionBlock[T]) error {
	if m.OnVerifyExpiryReplayProtection != nil {
		return m.OnVerifyExpiryReplayProtection(ctx, blk)
	}
	return nil
}

func (m *MockTimeValidityWindow[T]) IsRepeat(ctx context.Context, parentBlk validitywindow.ExecutionBlock[T], currentTimestamp int64, containers []T) (set.Bits, error) {
	if m.OnIsRepeat != nil {
		return m.OnIsRepeat(ctx, parentBlk, containers, currentTimestamp)
	}
	return set.NewBits(), nil
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/x/merkledb"
)

var _ PendingState = (*SimplePendingState)(nil)

type SimplePendingState struct {
	State

	changes map[string]*merkledb.ChangeOp
}

func NewSimplePendingState(s State) *SimplePendingState {
	return &SimplePendingState{s, make(map[string]*merkledb.ChangeOp)}
}

func (s *SimplePendingState) Insert(_ context.Context, k []byte, v []byte) error {
	s.changes[string(k)] = &merkledb.ChangeOp{Value: v, Delete: false}
	return nil
}

func (s *SimplePendingState) Remove(_ context.Context, k []byte) error {
	s.changes[string(k)] = &merkledb.ChangeOp{Value: nil, Delete: true}
	return nil
}

func (s *SimplePendingState) Commit(ctx context.Context) error {
	view, err := s.State.NewViewFromMap(ctx, s.changes, false)
	if err != nil {
		return err
	}
	return view.CommitToDB(ctx)
}

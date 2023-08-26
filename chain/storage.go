// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/x/merkledb"
)

var _ MutableState = (*SimpleMutableState)(nil)

type SimpleMutableState struct {
	State

	changes map[string]*merkledb.ChangeOp
}

func NewSimpleMutableState(s State) *SimpleMutableState {
	return &SimpleMutableState{s, make(map[string]*merkledb.ChangeOp)}
}

func (s *SimpleMutableState) Insert(_ context.Context, k []byte, v []byte) error {
	s.changes[string(k)] = &merkledb.ChangeOp{Value: v, Delete: false}
	return nil
}

func (s *SimpleMutableState) Remove(_ context.Context, k []byte) error {
	s.changes[string(k)] = &merkledb.ChangeOp{Value: nil, Delete: true}
	return nil
}

func (s *SimpleMutableState) Commit(ctx context.Context) error {
	view, err := s.State.NewViewFromMap(ctx, s.changes, false)
	if err != nil {
		return err
	}
	return view.CommitToDB(ctx)
}

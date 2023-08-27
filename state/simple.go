// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/x/merkledb"
)

var _ Mutable = (*SimpleMutable)(nil)

type SimpleMutable struct {
	v View

	changes map[string]*merkledb.ChangeOp
}

func NewSimpleMutable(v View) *SimpleMutable {
	return &SimpleMutable{v, make(map[string]*merkledb.ChangeOp)}
}

func (s *SimpleMutable) GetValue(ctx context.Context, k []byte) ([]byte, error) {
	if v, ok := s.changes[string(k)]; ok {
		if v.Delete {
			return nil, database.ErrNotFound
		}
		return v.Value, nil
	}
	return s.v.GetValue(ctx, k)
}

func (s *SimpleMutable) Insert(_ context.Context, k []byte, v []byte) error {
	s.changes[string(k)] = &merkledb.ChangeOp{Value: v, Delete: false}
	return nil
}

func (s *SimpleMutable) Remove(_ context.Context, k []byte) error {
	s.changes[string(k)] = &merkledb.ChangeOp{Value: nil, Delete: true}
	return nil
}

func (s *SimpleMutable) Commit(ctx context.Context) error {
	view, err := s.v.NewViewFromMap(ctx, s.changes, false)
	if err != nil {
		return err
	}
	return view.CommitToDB(ctx)
}

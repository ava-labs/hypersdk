// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"github.com/ava-labs/avalanchego/x/merkledb"
)

var _ Mutable = (*SimpleMutable)(nil)

type SimpleMutable struct {
	v View

	changes map[string]maybe.Maybe[[]byte]
}

func NewSimpleMutable(v View) *SimpleMutable {
	return &SimpleMutable{v, make(map[string]maybe.Maybe[[]byte])}
}

func (s *SimpleMutable) GetValue(ctx context.Context, k []byte) ([]byte, error) {
	if v, ok := s.changes[string(k)]; ok {
		if v.IsNothing() {
			return nil, database.ErrNotFound
		}
		return v.Value(), nil
	}
	return s.v.GetValue(ctx, k)
}

func (s *SimpleMutable) Insert(_ context.Context, k []byte, v []byte) error {
	s.changes[string(k)] = maybe.Some(v)
	return nil
}

func (s *SimpleMutable) Remove(_ context.Context, k []byte) error {
	s.changes[string(k)] = maybe.Nothing[[]byte]()
	return nil
}

func (s *SimpleMutable) Commit(ctx context.Context) error {
	view, err := s.v.NewView(ctx, merkledb.ViewChanges{MapOps: s.changes})
	if err != nil {
		return err
	}
	return view.CommitToDB(ctx)
}

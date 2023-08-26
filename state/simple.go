package state

import (
	"context"

	"github.com/ava-labs/avalanchego/x/merkledb"
)

var _ Mutable = (*SimpleMutable)(nil)

type SimpleMutable struct {
	View

	changes map[string]*merkledb.ChangeOp
}

func NewSimpleMutable(v View) *SimpleMutable {
	return &SimpleMutable{v, make(map[string]*merkledb.ChangeOp)}
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
	view, err := s.View.NewViewFromMap(ctx, s.changes, false)
	if err != nil {
		return err
	}
	return view.CommitToDB(ctx)
}

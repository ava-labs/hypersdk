// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/x/merkledb"
)

var _ Database = (*ReadOnlyDatabase)(nil)

type ReadOnlyDatabase struct {
	StateDatabase
}

func NewReadOnlyDatabase(db StateDatabase) Database {
	return &ReadOnlyDatabase{db}
}

func (*ReadOnlyDatabase) Insert(_ context.Context, _ []byte, _ []byte) error {
	return ErrModificationNotAllowed
}

func (*ReadOnlyDatabase) Remove(_ context.Context, _ []byte) error {
	return ErrModificationNotAllowed
}

var _ Database = (*SimpleDatabase)(nil)

type SimpleDatabase struct {
	StateDatabase

	changes map[string]*merkledb.ChangeOp
}

func NewSimpleDatabase(db StateDatabase) *SimpleDatabase {
	return &SimpleDatabase{db, make(map[string]*merkledb.ChangeOp)}
}

func (s *SimpleDatabase) Insert(_ context.Context, k []byte, v []byte) error {
	s.changes[string(k)] = &merkledb.ChangeOp{Value: v, Delete: false}
	return nil
}

func (s *SimpleDatabase) Remove(_ context.Context, k []byte) error {
	s.changes[string(k)] = &merkledb.ChangeOp{Value: nil, Delete: true}
	return nil
}

func (s *SimpleDatabase) Commit(ctx context.Context) error {
	view, err := s.StateDatabase.NewViewFromMap(ctx, s.changes, false)
	if err != nil {
		return err
	}
	return view.CommitToDB(ctx)
}

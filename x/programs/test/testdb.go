// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package test

import (
	"context"

	"github.com/ava-labs/avalanchego/database/memdb"

	"github.com/ava-labs/hypersdk/state"
)

var (
	_ state.Mutable   = (*DB)(nil)
	_ state.Immutable = (*DB)(nil)
)

func NewTestDB() *DB {
	return &DB{db: memdb.New()}
}

type DB struct {
	db *memdb.Database
}

func (c *DB) GetValue(_ context.Context, key []byte) ([]byte, error) {
	return c.db.Get(key)
}

func (c *DB) Insert(_ context.Context, key []byte, value []byte) error {
	return c.db.Put(key, value)
}

func (c *DB) Put(key []byte, value []byte) error {
	return c.db.Put(key, value)
}

func (c *DB) Remove(_ context.Context, key []byte) error {
	return c.db.Delete(key)
}

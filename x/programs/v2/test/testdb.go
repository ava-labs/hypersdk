// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package test

import (
	"context"

	"github.com/ava-labs/avalanchego/database/memdb"

	"github.com/ava-labs/hypersdk/state"
)

var (
	_ state.Mutable   = &testDB{}
	_ state.Immutable = &testDB{}
)

func newTestDB() *testDB {
	return &testDB{db: memdb.New()}
}

type testDB struct {
	db *memdb.Database
}

func (c *testDB) GetValue(_ context.Context, key []byte) ([]byte, error) {
	return c.db.Get(key)
}

func (c *testDB) Insert(_ context.Context, key []byte, value []byte) error {
	return c.db.Put(key, value)
}

func (c *testDB) Put(key []byte, value []byte) error {
	return c.db.Put(key, value)
}

func (c *testDB) Remove(_ context.Context, key []byte) error {
	return c.db.Delete(key)
}

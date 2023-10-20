// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"os"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

func newKeyPtr(ctx context.Context, key ed25519.PublicKey, runtime runtime.Runtime) (uint64, error) {
	ptr, err := runtime.Memory().Alloc(ed25519.PublicKeyLen)
	if err != nil {
		return 0, err
	}

	// write programID to memory which we will later pass to the program
	err = runtime.Memory().Write(ptr, key[:])
	if err != nil {
		return 0, err
	}

	return ptr, err
}

func newKey() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	priv, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return ed25519.EmptyPrivateKey, ed25519.EmptyPublicKey, err
	}

	return priv, priv.PublicKey(), nil
}

var (
	_ state.Mutable   = &testDB{}
	_ state.Immutable = &testDB{}
)

type testDB struct {
	db *memdb.Database
}

func newTestDB() *testDB {
	return &testDB{
		db: memdb.New(),
	}
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

func GetProgramBytes(filePath string) ([]byte, error) {
	return os.ReadFile(filePath)
}

func GetGuestFnName(name string) string {
	return name + "_guest"
}

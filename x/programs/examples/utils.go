// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"os"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/program"
	"github.com/near/borsh-go"
)

func newKey() (ed25519.PrivateKey, ed25519.PublicKey, error) {
	priv, err := ed25519.GeneratePrivateKey()
	if err != nil {
		return ed25519.EmptyPrivateKey, ed25519.EmptyPublicKey, err
	}

	return priv, priv.PublicKey(), nil
}

// SerializeParameter serializes [obj] using Borsh
func serializeParameter(obj interface{}) ([]byte, error) {
	bytes, err := borsh.Serialize(obj)
	return bytes, err
}

// Serialize the parameter and create a smart ptr
func argumentToSmartPtr(obj interface{}, memory *program.Memory) (program.SmartPtr, error) {
	bytes, err := serializeParameter(obj)
	if err != nil {
		return 0, err
	}

	return program.BytesToSmartPtr(bytes, memory)
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

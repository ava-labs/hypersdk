// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"encoding/binary"
	"os"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/near/borsh-go"
)

// newPtr allocates memory and writes [bytes] to it.
// If [prependLength] is true, it prepends the length of [bytes] as a uint32 to [bytes].
// It returns the pointer to the allocated memory.
func newPtr(ctx context.Context, bytes []byte, rt runtime.Runtime) (int64, error) {
	amountToAllocate := uint64(len(bytes))

	ptr, err := rt.Memory().Alloc(amountToAllocate)
	if err != nil {
		return 0, err
	}

	// write programID to memory which we will later pass to the program
	err = rt.Memory().Write(ptr, bytes)
	if err != nil {
		return 0, err
	}

	return int64(ptr), err
}

// serializeParameter serializes [obj]
// using Borsh and prepends its length as a uint32.
// Designed for serializing parameters passed to a WASM program.
func serializeParameter(obj interface{}) ([]byte, error) {
	bytes, err := borsh.Serialize(obj)
	if err != nil {
		return nil, err
	}

	return marshalArg(bytes), nil
}

// newParameterPtr serializes [obj] and allocates memory for it.
func newParameterPtr(ctx context.Context, obj interface{}, rt runtime.Runtime) (int64, error) {
	bytes, err := serializeParameter(obj)
	if err != nil {
		return 0, err
	}
	return newPtr(ctx, bytes, rt)
}

// marshalArg prepends the length of [arg] as a uint32 to [arg].
// This is required by the program inorder to grab the correct number
// of bytes from memory.
func marshalArg(arg []byte) []byte {
	// add length prefix to arg as big endian uint32
	argLen := len(arg)
	bytes := make([]byte, consts.Uint32Len+argLen)
	binary.BigEndian.PutUint32(bytes, uint32(argLen))
	copy(bytes[consts.Uint32Len:], arg)
	return bytes
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

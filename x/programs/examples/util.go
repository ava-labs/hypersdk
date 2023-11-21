// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

func newPtr(ctx context.Context, bytes []byte, rt runtime.Runtime) (int64, error) {
	amountToAllocate := uint64(len(bytes)) + 4
	ptr, err := rt.Memory().Alloc(amountToAllocate)
	if err != nil {
		return 0, err
	}

	// write programID to memory which we will later pass to the program
	fmt.Println("bytes in go", marshalArg(bytes[:]))
	err = rt.Memory().Write(ptr, marshalArg(bytes[:]))
	if err != nil {
		return 0, err
	}

	return int64(ptr), err
}

func marshalArg(arg []byte) []byte {
	// add length prefix to arg as big endian uint32
	argLen := len(arg)
	bytes := make([]byte, 4 + argLen)
	binary.BigEndian.PutUint32(bytes, uint32(argLen))
	copy(bytes[4:], arg)
	return bytes
}

// func marshalArgs(args ...[]byte) []byte {
// 	var bytes []byte
// 	for _, arg := range args {
// 		bytes = append(bytes, arg...)
// 	}
// 	return marshalArg(bytes)
// }

func marshalInts(ints ...int64) []byte {
	bytes := make([]byte, 4 * len(ints))
	for i, val := range ints {
		binary.BigEndian.PutUint32(bytes[i * 4:], uint32(val))
	}
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

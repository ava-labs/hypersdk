// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

var _ runtime.Storage = (*programStorage)(nil)

// newProgramStorage returns an instance of runtime storage used for examples
// and backed by memDb.
func newProgramStorage(db chain.Database) *programStorage {
	return &programStorage{
		db:            db,
		programPrefix: 0x0,
	}
}

type programStorage struct {
	db            chain.Database
	programPrefix byte
}

func (p *programStorage) Get(ctx context.Context, id uint32) (bool, uint32, []byte, error) {
	getProgramBytes(ctx, p.db, id, p.programPrefix)
	return getProgramBytes(ctx, p.db, id, p.programPrefix)
}

func (p *programStorage) Set(ctx context.Context, id uint32, owner uint32, data []byte) error {
	k := prefixProgramKey(p.programPrefix, id)
	return p.db.Insert(ctx, k, data)
}

// [programID] -> [exists, owner, payload]
func getProgram(
	db database.Database,
	programID ids.ID,
) (
	bool, // exists
	ed25519.PublicKey, // owner
	[]byte, // program bytes
	error,
) {
	k := programKey(programID)
	v, err := db.Get(k)
	if errors.Is(err, database.ErrNotFound) {
		return false, ed25519.EmptyPublicKey, nil, nil
	}
	if err != nil {
		return false, ed25519.EmptyPublicKey, nil, err
	}
	var owner ed25519.PublicKey
	copy(owner[:], v[ed25519.PublicKeyLen:])
	var program []byte
	copy(program[:], v[ed25519.PublicKeyLen:])
	return true, owner, program, nil
}

func getProgramBytes(
	ctx context.Context,
	db chain.Database,
	id uint32,
	prefix byte,
) ([]byte, bool, error) {
	k := prefixProgramKey(prefix, id)
	v, err := db.GetValue(ctx, k)
	if errors.Is(err, database.ErrNotFound) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return v, true, nil
}

func prefixProgramKey(prefix byte, asset uint32) (k []byte) {
	k = make([]byte, 1+consts.IDLen)
	k[0] = prefix
	binary.BigEndian.PutUint32(k, asset)
	return
}

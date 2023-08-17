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

var _ runtime.ProgramStorage = (*programStorage)(nil)

// newProgramStorage returns an instance of program storage used for examples
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

func (p *programStorage) Get(ctx context.Context, id uint32) ([]byte, bool, error) {
	return getProgramBytes(ctx, p.db, id, p.programPrefix)
}

func (p *programStorage) Set(ctx context.Context, id uint32, _ uint32, data []byte) error {
	k := prefixProgramKey(p.programPrefix, id)
	return p.db.Insert(ctx, k, data)
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

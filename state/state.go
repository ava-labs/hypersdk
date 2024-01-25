// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/x/merkledb"
)

const (
	ModeLen = 8
	Read    = 0
	Write   = 1
)

type Immutable interface {
	GetValue(ctx context.Context, key []byte) (value []byte, err error)
}

type Mutable interface {
	Immutable

	Insert(ctx context.Context, key []byte, value []byte) error
	Remove(ctx context.Context, key []byte) error
}

type View interface {
	Immutable

	NewView(ctx context.Context, changes merkledb.ViewChanges) (merkledb.View, error)
	GetMerkleRoot(ctx context.Context) (ids.ID, error)
}

type Key struct {
	Name string
	// TODO: consider a dynamically sized []byte
	Mode [ModeLen]byte
}

func NewKey(name string, bits ...int) Key {
	var key Key
	key.Name = name

	for _, bit := range bits {
		byteIdx := bit / 8
		bitIdx := uint(bit % 8)

		if byteIdx < ModeLen {
			key.Mode[byteIdx] |= (1 << bitIdx)
		}
	}

	return key
}

func (k *Key) HasMode(mode int) bool {
	byteIdx := mode / 8
	bitIdx := uint(mode % 8)

	if byteIdx >= ModeLen {
		return false
	}

	return (k.Mode[byteIdx] & (1 << bitIdx)) != 0
}

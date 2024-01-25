// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/x/merkledb"
)

const (
	PermissionLen = 8
	Read          = 0
	Write         = 1
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
	Permission [PermissionLen]byte
}

func NewKey(name string, permissions ...int) Key {
	var key Key
	key.Name = name

	for _, perm := range permissions {
		byteIdx := perm / 8
		bitIdx := uint(perm % 8)

		if byteIdx < PermissionLen {
			key.Permission[byteIdx] |= (1 << bitIdx)
		}
	}

	return key
}

func (k *Key) HasPermission(permission int) bool {
	byteIdx := permission / 8
	bitIdx := uint(permission % 8)

	if byteIdx >= PermissionLen {
		return false
	}

	return (k.Permission[byteIdx] & (1 << bitIdx)) == 1
}

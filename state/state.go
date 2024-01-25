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

// StateKey holds the name of the key and its permission (Read/Write)
// Specifying RWrite is setting both the Read and Write bit
type Key struct {
	Name string
	// TODO: consider a dynamically sized []byte
	Permission Permission
}

// TODO: Support Alloc bit
type Permission [PermissionLen]byte

// Returns a StateKey with the appropriate permission bits set
// By default, no permission bits are set
func NewKey(name string, permissions ...int) Key {
	var key Key
	key.Name = name
	for _, perm := range permissions {
		// Only set bit if it's within the byte slice
		if perm < PermissionLen {
			key.Permission[perm] |= 1
		}
	}
	return key
}

// Checks whether a StateKey has the appropriate permission
// to perform a certain access (Read/Write).
func (p *Permission) HasPermission(permission int) bool {
	// Trying to access a bit out of bounds
	if permission >= PermissionLen {
		return false
	}
	return p[permission] == 1
}

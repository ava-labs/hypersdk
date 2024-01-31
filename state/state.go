// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
)

const (
	Read          uint8 = 0
	Write         uint8 = 1
	PermissionLen uint8 = 8 // Sufficient to use a single byte
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

// StateKey holds the name of the key and its permission (Read/Write). By default,
// initialization of Keys with duplicate key will not work. And to prevent duplicate
// insertions from overriding the original permissions, use the Add function below.
// Permission uses bits since transactions are expected to include this encoding
type Keys map[string]Permission

// The access permission (Read/Write) for each StateKey
// Specifying RWrite is setting both the Read and Write bit
type Permission set.Bits

// By default, no permission bits are set
func NewPermission(permissions ...uint8) Permission {
	withinBoundsPermission := []int{} // NewBits requires int type
	for _, v := range permissions {
		// Only set bit to 1 if we're within 8 bits
		if v < PermissionLen {
			withinBoundsPermission = append(withinBoundsPermission, int(v))
		}
	}
	return Permission(set.NewBits(withinBoundsPermission...))
}

// Checks whether a StateKey has the appropriate permission
// to perform a certain access (Read/Write).
func (p Permission) HasPermission(permission uint8) bool {
	return set.Bits(p).Contains(int(permission))
}

// Transactions are expected to use this to prevent
// overriding of key permissions
func (k Keys) Add(name string, permission Permission) {
	_, exists := k[name]
	if !exists {
		k[name] = permission
	} else {
		// Transaction's permissions are the union of all
		// state keys from both Actions and Auth
		set.Bits(k[name]).Union(set.Bits(permission))
	}
}

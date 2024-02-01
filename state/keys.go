// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import "github.com/ava-labs/avalanchego/utils/set"

const (
	Read PermissionBit = 0
	// TODO: Handle Allocate permission
	Allocate      PermissionBit = 1
	Write         PermissionBit = 2
	PermissionLen PermissionBit = 8 // Sufficient to use a single byte
)

// StateKey holds the name of the key and its permission (Read/Allocate/Write). By default,
// initialization of Keys with duplicate key will not work. And to prevent duplicate
// insertions from overriding the original permissions, use the Add function below.
// Permissions uses bits since transactions are expected to include this encoding
type Keys map[string]Permissions

// The access permission (Read/Allocate/Write) for each StateKey
// Specifying RWrite is setting both the Read and Write bit
type Permissions set.Bits

type PermissionBit int

// By default, no permission bits are set. Note that any
// undefined permissions are dropped
func Permission(permissions ...PermissionBit) Permissions {
	perms := set.NewBits()
	for _, v := range permissions {
		if v < PermissionLen {
			// Only set bit to 1 if we're within 8 bits
			perms.Add(int(v))
		}
	}
	return Permissions(perms)
}

// Checks whether a StateKey has the appropriate permission
// to perform a certain access (Read/Allocate/Write).
func (p Permissions) HasPermission(permission PermissionBit) bool {
	return set.Bits(p).Contains(int(permission))
}

// Transactions are expected to use this to prevent
// overriding of key permissions
func (k Keys) Add(name string, permission Permissions) {
	_, exists := k[name]
	if !exists {
		k[name] = permission
	} else {
		// Transaction's permissions are the union of all
		// state keys from both Actions and Auth
		set.Bits(k[name]).Union(set.Bits(permission))
	}
}

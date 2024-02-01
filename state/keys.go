// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import "github.com/ava-labs/avalanchego/utils/set"

const (
	Read PermissionBit = 0
	// TODO: Handle Allocate permission
	Allocate PermissionBit = 1
	Write    PermissionBit = 2
)

// We use a bitset (8 bits) in the Action and Auth marshal/unmarshaling.
type PermissionsBits set.Bits

// Bit representation of Read/Allocate/Write
type PermissionBit int

// StateKey holds the name of the key and its permission (Read/Allocate/Write). By default,
// initialization of Keys with duplicate key will not work. And to prevent duplicate
// insertions from overriding the original permissions, use the Add function below.
type Keys map[string]*Permissions

// All acceptable permission options
type Permissions struct {
	Read     bool
	Allocate bool
	Write    bool
}

// Transactions are expected to use this to prevent
// overriding of key permissions
func (k Keys) Add(name string, permission *Permissions) {
	_, exists := k[name]
	if !exists {
		k[name] = permission
	} else {
		// Transaction's permissions are the union of all
		// state keys from both Actions and Auth
		if permission.Read {
			k[name].Read = true
		}
		if permission.Allocate {
			k[name].Allocate = true
		}
		if permission.Write {
			k[name].Write = true
		}
	}
}

// Populates bitset using the values from Permissions struct
// We don't need to worry about adding invalid permission bits
// since our struct is defined with all valid ones
func (p Permissions) ToBytes() []byte {
	permBitSet := set.NewBits()
	if p.Read {
		permBitSet.Add(int(Read))
	}
	if p.Allocate {
		permBitSet.Add(int(Allocate))
	}
	if p.Write {
		permBitSet.Add(int(Write))
	}
	return permBitSet.Bytes()
}

// Populates Permissions struct with the permissions bytes
// Any permission bits that were not defined will be dropped
// By default, all fields in Permissions are false
func FromBytes(permissionBytes []byte) *Permissions {
	var permissions Permissions
	permissionBits := set.BitsFromBytes(permissionBytes)
	if permissionBits.Contains(int(Read)) {
		permissions.Read = true
	}
	if permissionBits.Contains(int(Allocate)) {
		permissions.Allocate = true
	}
	if permissionBits.Contains(int(Write)) {
		permissions.Write = true
	}
	return &permissions
}

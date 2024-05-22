// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import "github.com/ava-labs/hypersdk/keys"

const (
	Read     Permissions = 1
	Allocate             = 1<<1 | Read
	Write                = 1<<2 | Read

	None Permissions = 0
	All              = Read | Allocate | Write
)

// StateKey holds the name of the key and its permission (Read/Allocate/Write). By default,
// initialization of Keys with duplicate key will not work. And to prevent duplicate
// insertions from overriding the original permissions, use the Add function below.
type Keys map[string]Permissions

// All acceptable permission options
type Permissions byte

// Add verifies that a key is well-formatted and adds it to the conflict set.
//
// If the key already exists, the permissions are unioned.
func (k Keys) Add(key string, permission Permissions) bool {
	// If a key is not properly formatted, it cannot be added.
	if !keys.Valid(key) {
		return false
	}

	// Transaction's permissions are the union of all
	// state keys from both Actions and Auth
	k[key] |= permission
	return true
}

// Has returns true if [p] has all the permissions that are contained in require
func (p Permissions) Has(require Permissions) bool {
	return require&^p == 0
}

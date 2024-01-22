// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package types

type Key struct {
	Name string
	Mode KeyMode
}

type KeyMode uint8

const (
	// These `chain` consts are defined here to avoid circular dependency
	Read   KeyMode = 0
	Write  KeyMode = 1
	RWrite KeyMode = 2
)

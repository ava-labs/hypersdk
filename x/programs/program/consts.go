// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"github.com/ava-labs/avalanchego/utils/units"
)

const (
	AllocFnName    = "alloc"
	DeallocFnName  = "dealloc"
	MemoryFnName   = "memory"
	GuestSuffix    = "_guest"
	MemoryPageSize = 64 * units.KiB
)

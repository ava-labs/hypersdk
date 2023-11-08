// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"math"

	"github.com/ava-labs/avalanchego/utils/units"
)

const (
	AllocFnName         = "alloc"
	DeallocFnName       = "dealloc"
	MemoryFnName        = "memory"
	guestSuffix         = "_guest"
	wasiPreview1ModName = "wasi_snapshot_preview1"
	MemoryPageSize      = 64 * units.KiB
	MaxInt64            = math.MaxInt64
	MinInt64            = math.MinInt64
)

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"time"

	"github.com/ava-labs/avalanchego/utils/units"
)

const (
	FutureBound        = 10 * time.Second
	MaxWarpMessageSize = 256 * units.KiB
	// MaxWarpMessages is the maximum number of warp messages allows in a single
	// block.
	MaxWarpMessages = 64
)

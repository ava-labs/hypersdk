// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"time"

	"github.com/ava-labs/avalanchego/utils/units"
)

const (
	// FutureBound is used to reject blocks that are produced....
	//
	// This value should be (much) less than the value of [ProposerWindow], otherwise honest
	// nodes may not build during their allocated window (to avoid producing ahead of current time).
	//
	// TODO: finish comments
	FutureBound        = 1 * time.Second
	MaxWarpMessageSize = 256 * units.KiB
	// MaxWarpMessages is the maximum number of warp messages allows in a single
	// block.
	MaxWarpMessages = 64
)

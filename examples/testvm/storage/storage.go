// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import (
	"github.com/ava-labs/hypersdk/state/metadata"
)

// State
// 0x0/ (hypersdk-height)
// 0x1/ (hypersdk-timestamp)
// 0x2/ (hypersdk-fee)
//
// 0x3/ (balance)
//
//	-> [owner] => balance
//
// 0x4/ (counter)
//
//	-> [owner] => count
const (
	counterPrefix byte = metadata.DefaultMinimumPrefix + 1
)

const (
	CounterChunks uint16 = 1
)

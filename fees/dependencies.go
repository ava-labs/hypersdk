// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"github.com/ava-labs/avalanchego/ids"
)

type Rules interface {
	// Should almost always be constant (unless there is a fork of
	// a live network)
	NetworkID() uint32
	ChainID() ids.ID

	GetMinBlockGap() int64      // in milliseconds
	GetMinEmptyBlockGap() int64 // in milliseconds
	GetValidityWindow() int64   // in milliseconds

	GetMinUnitPrice() Dimensions
	GetUnitPriceChangeDenominator() Dimensions
	GetWindowTargetUnits() Dimensions
	GetMaxBlockUnits() Dimensions

	GetBaseComputeUnits() uint64
	GetBaseWarpComputeUnits() uint64
	GetWarpComputeUnitsPerSigner() uint64
	GetOutgoingWarpComputeUnits() uint64

	// Invariants:
	// * Controllers must manage the max key length and max value length (max network
	//   limit is ~2MB)
	// * Creating a new key involves first allocating and then writing
	// * Keys are only charged once per transaction (even if used multiple times), it is
	//   up to the controller to ensure multiple usage has some compute cost
	//
	// Interesting Scenarios:
	// * If a key is created and then modified during a transaction, the second
	//   read will be a read of 0 chunks (reads are based on disk contents before exec)
	// * If a key is removed and then re-created with the same value during a transaction,
	//   it doesn't count as a modification (returning to the current value on-disk is a no-op)
	GetSponsorStateKeysMaxChunks() []uint16
	GetStorageKeyReadUnits() uint64
	GetStorageValueReadUnits() uint64 // per chunk
	GetStorageKeyAllocateUnits() uint64
	GetStorageValueAllocateUnits() uint64 // per chunk
	GetStorageKeyWriteUnits() uint64
	GetStorageValueWriteUnits() uint64 // per chunk

	GetWarpConfig(sourceChainID ids.ID) (bool, uint64, uint64)

	FetchCustom(string) (any, bool)
}
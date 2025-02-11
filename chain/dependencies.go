// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/state"
)

type Mempool[T Action[T], A Auth[A]] interface {
	Len(context.Context) int  // items
	Size(context.Context) int // bytes
	Add(context.Context, []*Transaction[T, A])

	StartStreaming(context.Context)
	PrepareStream(context.Context, int)
	Stream(context.Context, int) []*Transaction[T, A]
	FinishStreaming(context.Context, []*Transaction[T, A]) int
}

// TODO: add fixed rules as a subset of this interface
type Rules interface {
	// Should almost always be constant (unless there is a fork of
	// a live network)
	GetNetworkID() uint32
	GetChainID() ids.ID

	GetMinBlockGap() int64      // in milliseconds
	GetMinEmptyBlockGap() int64 // in milliseconds
	GetValidityWindow() int64   // in milliseconds

	GetMaxActionsPerTx() uint8

	GetMinUnitPrice() fees.Dimensions
	GetUnitPriceChangeDenominator() fees.Dimensions
	GetWindowTargetUnits() fees.Dimensions
	GetMaxBlockUnits() fees.Dimensions

	GetBaseComputeUnits() uint64

	// Invariants:
	// * VMs must manage the max key length and max value length (max network
	//   limit is ~2MB)
	// * Creating a new key involves first allocating and then writing
	// * Keys are only charged once per transaction (even if used multiple times), it is
	//   up to the controller to ensure multiple usage has some compute cost
	GetSponsorStateKeysMaxChunks() []uint16
	GetStorageKeyReadUnits() uint64
	GetStorageValueReadUnits() uint64 // per chunk
	GetStorageKeyAllocateUnits() uint64
	GetStorageValueAllocateUnits() uint64 // per chunk
	GetStorageKeyWriteUnits() uint64
	GetStorageValueWriteUnits() uint64 // per chunk

	FetchCustom(string) (any, bool)
}

type RuleFactory interface {
	GetRules(t int64) Rules
}

type MetadataManager interface {
	HeightPrefix() []byte
	TimestampPrefix() []byte
	FeePrefix() []byte
}

type BalanceHandler interface {
	// SponsorStateKeys is a full enumeration of all database keys that could be touched during fee payment
	// by [addr]. This is used to prefetch state and will be used to parallelize execution (making
	// an execution tree is trivial).
	//
	// All keys specified must be suffixed with the number of chunks that could ever be read from that
	// key (formatted as a big-endian uint16). This is used to automatically calculate storage usage.
	SponsorStateKeys(addr codec.Address) state.Keys

	// CanDeduct returns an error if [amount] cannot be paid by [addr].
	CanDeduct(ctx context.Context, addr codec.Address, im state.Immutable, amount uint64) error

	// Deduct removes [amount] from [addr] during transaction execution to pay fees.
	Deduct(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64) error

	// AddBalance adds [amount] to [addr].
	AddBalance(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64) error

	// GetBalance returns the balance of [addr].
	// If [addr] does not exist, this should return 0 and no error.
	GetBalance(ctx context.Context, addr codec.Address, im state.Immutable) (uint64, error)
}

type AuthBatchVerifier[A Auth[A]] interface {
	Add([]byte, A) func() error
	Done() []func() error
}

type ValidityWindow[T Action[T], A Auth[A]] interface {
	VerifyExpiryReplayProtection(
		ctx context.Context,
		blk validitywindow.ExecutionBlock[*Transaction[T, A]],
	) error
	Accept(blk validitywindow.ExecutionBlock[*Transaction[T, A]])
	IsRepeat(
		ctx context.Context,
		parentBlk validitywindow.ExecutionBlock[*Transaction[T, A]],
		currentTimestamp int64,
		txs []*Transaction[T, A],
	) (set.Bits, error)
}

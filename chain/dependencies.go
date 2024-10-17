// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/executor"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
)

type Parser interface {
	Rules(int64) Rules
	ActionCodec() *codec.TypeParser[Action]
	OutputCodec() *codec.TypeParser[codec.Typed]
	AuthCodec() *codec.TypeParser[Auth]
}

type Metrics interface {
	RecordRootCalculated(time.Duration) // only called in Verify
	RecordWaitRoot(time.Duration)       // only called in Verify
	RecordWaitSignatures(time.Duration) // only called in Verify

	RecordBlockVerify(time.Duration)
	RecordBlockAccept(time.Duration)
	RecordStateChanges(int)
	RecordStateOperations(int)
	RecordBuildCapped()
	RecordEmptyBlockBuilt()
	RecordClearedMempool()
	GetExecutorBuildRecorder() executor.Metrics
	GetExecutorVerifyRecorder() executor.Metrics
}

type Monitoring interface {
	Tracer() trace.Tracer
	Logger() logging.Logger
}

type VM interface {
	Metrics
	Monitoring
	Parser

	// We don't include this in registry because it would never be used
	// by any client of the hypersdk.
	AuthVerifiers() workers.Workers
	GetAuthBatchVerifier(authTypeID uint8, cores int, count int) (AuthBatchVerifier, bool)
	GetVerifyAuth() bool

	IsBootstrapped() bool
	LastAcceptedBlock() *StatefulBlock
	GetStatefulBlock(context.Context, ids.ID) (*StatefulBlock, error)

	State() (merkledb.MerkleDB, error)
	BalanceHandler() BalanceHandler
	MetadataManager() MetadataManager

	Mempool() Mempool
	IsRepeat(context.Context, []*Transaction, set.Bits, bool) set.Bits
	GetTargetBuildDuration() time.Duration
	GetTransactionExecutionCores() int
	GetStateFetchConcurrency() int

	Verified(context.Context, *StatefulBlock)
	Rejected(context.Context, *StatefulBlock)
	Accepted(context.Context, *StatefulBlock)
	AcceptedSyncableBlock(context.Context, *SyncableBlock) (block.StateSyncMode, error)

	// UpdateSyncTarget returns a bool that is true if the root
	// was updated and the sync is continuing with the new specified root
	// and false if the sync completed with the previous root.
	UpdateSyncTarget(*StatefulBlock) (bool, error)
	StateReady() bool
}

type VerifyContext interface {
	View(ctx context.Context, verify bool) (state.View, error)
	// IsRepeat returns a bitset containing the indices of [txs] that are repeats from this context back to
	// [oldestAllowed].
	// If [stop] is true, the search will stop at the first repeat transaction. This supports early termination
	// during verification when any invalid transaction will cause the block to fail verification.
	IsRepeat(ctx context.Context, oldestAllowed int64, txs []*Transaction, marker set.Bits, stop bool) (set.Bits, error)
}

type Mempool interface {
	Len(context.Context) int  // items
	Size(context.Context) int // bytes
	Add(context.Context, []*Transaction)

	Top(
		context.Context,
		time.Duration,
		func(context.Context, *Transaction) (cont bool, rest bool, err error),
	) error

	StartStreaming(context.Context)
	PrepareStream(context.Context, int)
	Stream(context.Context, int) []*Transaction
	FinishStreaming(context.Context, []*Transaction) int
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
	// * Controllers must manage the max key length and max value length (max network
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

type MetadataManager interface {
	HeightPrefix() []byte
	TimestampPrefix() []byte
	FeePrefix() []byte
}

type BalanceHandler interface {
	// StateKeys is a full enumeration of all database keys that could be touched during fee payment
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
	AddBalance(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64, createAccount bool) error
}

type Object interface {
	// GetTypeID uniquely identifies each supported [Action]. We use IDs to avoid
	// reflection.
	GetTypeID() uint8

	// ValidRange is the timestamp range (in ms) that this [Action] is considered valid.
	//
	// -1 means no start/end
	ValidRange(Rules) (start int64, end int64)
}

type Marshaler interface {
	// Marshal encodes a type as bytes.
	Marshal(p *codec.Packer)
	// Size is the number of bytes it takes to represent this [Action]. This is used to preallocate
	// memory during encoding and to charge bandwidth fees.
	Size() int
}

type Action interface {
	Object

	// ComputeUnits is the amount of compute required to call [Execute]. This is used to determine
	// whether the [Action] can be included in a given block and to compute the required fee to execute.
	ComputeUnits(Rules) uint64

	// StateKeys is a full enumeration of all database keys that could be touched during execution
	// of an [Action]. This is used to prefetch state and will be used to parallelize execution (making
	// an execution tree is trivial).
	//
	// All keys specified must be suffixed with the number of chunks that could ever be read from that
	// key (formatted as a big-endian uint16). This is used to automatically calculate storage usage.
	//
	// If any key is removed and then re-created, this will count as a creation instead of a modification.
	StateKeys(actor codec.Address) state.Keys

	// Execute actually runs the [Action]. Any state changes that the [Action] performs should
	// be done here.
	//
	// If any keys are touched during [Execute] that are not specified in [StateKeys], the transaction
	// will revert and the max fee will be charged.
	//
	// If [Execute] returns an error, execution will halt and any state changes will revert.
	Execute(
		ctx context.Context,
		r Rules,
		mu state.Mutable,
		timestamp int64,
		actor codec.Address,
		actionID ids.ID,
	) (codec.Typed, error)
}

type Auth interface {
	Object
	Marshaler

	// ComputeUnits is the amount of compute required to call [Verify]. This is
	// used to determine whether [Auth] can be included in a given block and to compute
	// the required fee to execute.
	ComputeUnits(Rules) uint64

	// Verify is run concurrently during transaction verification. It may not be run by the time
	// a transaction is executed but will be checked before a [Transaction] is considered successful.
	// Verify is typically used to perform cryptographic operations.
	Verify(ctx context.Context, msg []byte) error

	// Actor is the subject of the [Action] signed.
	//
	// To avoid collisions with other [Auth] modules, this must be prefixed
	// by the [TypeID].
	Actor() codec.Address

	// Sponsor is the fee payer of the [Action] signed.
	//
	// If the [Actor] is not the same as [Sponsor], it is likely that the [Actor] signature
	// is wrapped by the [Sponsor] signature. It is important that the [Actor], in this case,
	// signs the [Sponsor] address or else their transaction could be replayed.
	//
	// TODO: add a standard sponsor wrapper auth (but this does not need to be handled natively)
	//
	// To avoid collisions with other [Auth] modules, this must be prefixed
	// by the [TypeID].
	Sponsor() codec.Address
}

type AuthBatchVerifier interface {
	Add([]byte, Auth) func() error
	Done() []func() error
}

type AuthFactory interface {
	// Sign is used by helpers, auth object should store internally to be ready for marshaling
	Sign(msg []byte) (Auth, error)
	MaxUnits() (bandwidth uint64, compute uint64)
	Address() codec.Address
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/avalanchego/x/merkledb"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/executor"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/workers"
)

type (
	ActionRegistry *codec.TypeParser[Action, *warp.Message, bool]
	SignerRegistry *codec.TypeParser[Signer, *warp.Message, bool]
)

type Parser interface {
	Rules(int64) Rules
	Registry() (ActionRegistry, SignerRegistry)
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
	SignerVerifiers() workers.Workers
	GetSignerBatchVerifier(signerTypeID uint8, cores int, count int) (SignerBatchVerifier, bool)
	GetVerifySigners() bool

	IsBootstrapped() bool
	LastAcceptedBlock() *StatelessBlock
	GetStatelessBlock(context.Context, ids.ID) (*StatelessBlock, error)

	GetVerifyContext(ctx context.Context, blockHeight uint64, parent ids.ID) (VerifyContext, error)

	State() (merkledb.MerkleDB, error)
	StateManager() StateManager
	ValidatorState() validators.State

	Mempool() Mempool
	IsRepeat(context.Context, []*Transaction, set.Bits, bool) set.Bits
	GetTargetBuildDuration() time.Duration
	GetTransactionExecutionCores() int

	Verified(context.Context, *StatelessBlock)
	Rejected(context.Context, *StatelessBlock)
	Accepted(context.Context, *StatelessBlock)
	AcceptedSyncableBlock(context.Context, *SyncableBlock) (block.StateSyncMode, error)

	// UpdateSyncTarget returns a bool that is true if the root
	// was updated and the sync is continuing with the new specified root
	// and false if the sync completed with the previous root.
	UpdateSyncTarget(*StatelessBlock) (bool, error)
	StateReady() bool
}

type VerifyContext interface {
	View(ctx context.Context, verify bool) (state.View, error)
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
	GetStorageKeyReadUnits() uint64
	GetStorageValueReadUnits() uint64 // per chunk
	GetStorageKeyAllocateUnits() uint64
	GetStorageValueAllocateUnits() uint64 // per chunk
	GetStorageKeyWriteUnits() uint64
	GetStorageValueWriteUnits() uint64 // per chunk

	GetWarpConfig(sourceChainID ids.ID) (bool, uint64, uint64)

	FetchCustom(string) (any, bool)
}

// StateManager allows [Chain] to safely store certain types of items in state
// in a structured manner. If we did not use [StateManager], we may overwrite
// state written by actions or auth.
//
// None of these keys should be suffixed with the max amount of chunks they will
// use. This will be handled by the hypersdk.
type StateManager interface {
	HeightKey() []byte
	TimestampKey() []byte
	FeeKey() []byte

	IncomingWarpKeyPrefix(sourceChainID ids.ID, msgID ids.ID) []byte
	OutgoingWarpKeyPrefix(txID ids.ID) []byte
}

type Object interface {
	// GetTypeID uniquely identifies each supported [Action]. We use IDs to avoid
	// reflection.
	GetTypeID() uint8

	// ValidRange is the timestamp range (in ms) that this [Action] is considered valid.
	//
	// -1 means no start/end
	ValidRange(Rules) (start int64, end int64)

	// Marshal encodes an [Action] as bytes.
	Marshal(p *codec.Packer)

	// Size is the number of bytes it takes to represent this [Action]. This is used to preallocate
	// memory during encoding and to charge bandwidth fees.
	Size() int
}

type Action interface {
	Object

	// MaxComputeUnits is the maximum amount of compute a given [Action] could use. This is
	// used to determine whether the [Action] can be included in a given block and to compute
	// the required fee to execute.
	//
	// Developers should make every effort to bound this as tightly to the actual max so that
	// users don't need to have a large balance to call an [Action] (must prepay fee before execution).
	MaxComputeUnits(Rules) uint64

	// StateKeysMaxChunks is used to estimate the fee a transaction should pay. It includes the max
	// chunks each state key could use without requiring the state keys to actually be provided (may
	// not be known until execution).
	StateKeysMaxChunks() []uint16

	// StateKeys is a full enumeration of all database keys that could be touched during execution
	// of an [Action]. This is used to prefetch state and will be used to parallelize execution (making
	// an execution tree is trivial).
	//
	// All keys specified must be suffixed with the number of chunks that could ever be read from that
	// key (formatted as a big-endian uint16). This is used to automatically calculate storage usage.
	//
	// If any key is removed and then re-created, this will count as a creation instead of a modification.
	StateKeys(actor codec.Address, txID ids.ID) []string

	// Execute actually runs the [Action]. Any state changes that the [Action] performs should
	// be done here.
	//
	// If any keys are touched during [Execute] that are not specified in [StateKeys], the transaction
	// will revert and the max fee will be charged.
	//
	// An error should only be returned if a fatal error was encountered, otherwise [success] should
	// be marked as false and fees will still be charged.
	Execute(
		ctx context.Context,
		r Rules,
		mu state.Mutable,
		timestamp int64,
		actor codec.Address,
		txID ids.ID,
		warpVerified bool,
	) (success bool, computeUnits uint64, output []byte, warpMessage *warp.UnsignedMessage, err error)

	// OutputsWarpMessage indicates whether an [Action] will produce a warp message. The max size
	// of any warp message is [MaxOutgoingWarpChunks].
	OutputsWarpMessage() bool
}

type Signer interface {
	// ComputeUnits is the amount of compute required to call [Verify]. This is
	// used to determine whether the [signer] can be included in a given block and to compute
	// the required fee to execute.
	ComputeUnits(Rules) uint64

	// Verify is run concurrently during transaction verification. It may not be run by the time
	// a transaction is executed but will be checked before a [Transaction] is considered successful.
	// Verify is typically used to perform cryptographic operations.
	Verify(msg []byte) error

	// Address is the unique identifier of the [Signer].
	//
	// To avoid collisions with other [Signer] modules, this must be prefixed
	// by the [TypeID].
	Address() codec.Address
}

type FeeHandler interface {
	// ComputeUnits is the amount of compute required to process fees.
	ComputeUnits(addr codec.Address, rules Rules) uint64

	// StateKeys is a full enumeration of all database keys that could be touched during fee payment
	// by [addr]. This is used to prefetch state and will be used to parallelize execution (making
	// an execution tree is trivial).
	//
	// All keys specified must be suffixed with the number of chunks that could ever be read from that
	// key (formatted as a big-endian uint16). This is used to automatically calculate storage usage.
	StateKeys(addr codec.Address) []string

	// CanDeduct returns an error if [amount] cannot be paid by [addr].
	CanDeduct(ctx context.Context, addr codec.Address, im state.Immutable, amount uint64) error

	// Deduct removes [amount] from [addr] during transaction execution to pay fees.
	Deduct(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64) error

	// Refund returns [amount] to [addr] after transaction execution if any fees were
	// not used.
	//
	// Refund will return an error if it attempts to create any new keys. It can only
	// modify or remove existing keys.
	//
	// Refund is only invoked if [amount] > 0.
	Refund(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64) error
}

type SignerBatchVerifier interface {
	Add([]byte, Signer) func() error
	Done() []func() error
}

type SignerFactory interface {
	// Sign is used by helpers, auth object should store internally to be ready for marshaling
	Sign(msg []byte, action Action) (Signer, error)

	MaxUnits() (bandwidth uint64, compute uint64, stateKeysCount []uint16)
}

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
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/workers"
)

type (
	ActionRegistry *codec.TypeParser[Action, *warp.Message, bool]
	AuthRegistry   *codec.TypeParser[Auth, *warp.Message, bool]
)

type Parser interface {
	Rules(int64) Rules
	Registry() (ActionRegistry, AuthRegistry)
}

type VM interface {
	Parser

	Workers() workers.Workers
	Tracer() trace.Tracer
	Logger() logging.Logger

	// We don't include this in registry because it would never be used
	// by any client of the hypersdk.
	GetAuthBatchVerifier(authTypeID uint8, cores int, count int) (AuthBatchVerifier, bool)

	IsBootstrapped() bool
	LastAcceptedBlock() *StatelessBlock
	SetLastAccepted(*StatelessBlock) error
	GetStatelessBlock(context.Context, ids.ID) (*StatelessBlock, error)

	State() (merkledb.MerkleDB, error)
	StateManager() StateManager
	ValidatorState() validators.State

	Mempool() Mempool
	IsRepeat(context.Context, []*Transaction, set.Bits, bool) set.Bits
	GetTargetBuildDuration() time.Duration

	Verified(context.Context, *StatelessBlock)
	Rejected(context.Context, *StatelessBlock)
	Accepted(context.Context, *StatelessBlock)
	AcceptedSyncableBlock(context.Context, *SyncableBlock) (block.StateSyncMode, error)

	// UpdateSyncTarget returns a bool that is true if the root
	// was updated and the sync is continuing with the new specified root
	// and false if the sync completed with the previous root.
	UpdateSyncTarget(*StatelessBlock) (bool, error)
	StateReady() bool

	// Collect useful metrics
	//
	// TODO: break out into own interface
	RecordRootCalculated(time.Duration) // only called in Verify
	RecordWaitSignatures(time.Duration) // only called in Verify
	RecordBlockVerify(time.Duration)
	RecordBlockAccept(time.Duration)
	RecordStateChanges(int)
	RecordStateOperations(int)
	RecordBuildCapped()
	RecordEmptyBlockBuilt()
	RecordClearedMempool()
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
	// * Cold key reads/modifications are assumed to be more expensive than warm
	//   when estimating the max fee
	// * Keys are only charged once per transaction (even if used multiple times), it is
	//   up to the controller to ensure multiple usage has some compute cost
	//
	// Interesting Scenarios:
	// * If a key is created and then modified during a transaction, the second
	//   read will be a warm read of 0 chunks (reads are based on disk contents before exec)
	// * If a key is removed and then re-created during a transaction, it counts
	//   as a modification and a creation instead of just a modification
	GetColdStorageKeyReadUnits() uint64
	GetColdStorageValueReadUnits() uint64 // per chunk
	GetWarmStorageKeyReadUnits() uint64
	GetWarmStorageValueReadUnits() uint64 // per chunk
	GetStorageKeyCreateUnits() uint64
	GetStorageValueCreateUnits() uint64 // per chunk
	GetColdStorageKeyModificationUnits() uint64
	GetColdStorageValueModificationUnits() uint64 // per chunk
	GetWarmStorageKeyModificationUnits() uint64
	GetWarmStorageValueModificationUnits() uint64 // per chunk

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
	FeeKey() []byte

	IncomingWarpKeyPrefix(sourceChainID ids.ID, msgID ids.ID) []byte
	OutgoingWarpKeyPrefix(txID ids.ID) []byte
}

type Action interface {
	// GetTypeID uniquely identifies each supported [Action]. We use IDs to avoid
	// reflection.
	GetTypeID() uint8

	// ValidRange is the timestamp range (in ms) that this [Action] is considered valid.
	//
	// -1 means no start/end
	ValidRange(Rules) (start int64, end int64)

	// MaxComputeUnits is the maximum amount of compute a given [Action] could use. This is
	// used to determine whether the [Action] can be included in a given block and to compute
	// the required fee to execute.
	//
	// Developers should make every effort to bound this as tightly to the actual max so that
	// users don't need to have a large balance to call an [Action] (must prepay fee before execution).
	MaxComputeUnits(Rules) uint64

	// OutputsWarpMessage indicates whether an [Action] will produce a warp message. The max size
	// of any warp message is [MaxOutgoingWarpChunks].
	OutputsWarpMessage() bool

	// StateKeys is a full enumeration of all database keys that could be touched during execution
	// of an [Action]. This is used to prefetch state and will be used to parallelize execution (making
	// an execution tree is trivial).
	//
	// All keys specified must be suffixed with the number of chunks that could ever be read from that
	// key (formatted as a big-endian uint16). This is used to automatically calculate storage usage.
	//
	// If any key is removed and then re-created, this will count as a creation instead of a modification.
	StateKeys(auth Auth, txID ids.ID) []string

	// StateKeysMaxChunks is used to estimate the fee a transaction should pay. It includes the max
	// chunks each state key could use without requiring the state keys to actually be provided (may
	// not be known until execution).
	StateKeysMaxChunks() []uint16

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
		auth Auth,
		txID ids.ID,
		warpVerified bool,
	) (success bool, computeUnits uint64, output []byte, warpMessage *warp.UnsignedMessage, err error)

	// Marshal encodes an [Action] as bytes.
	Marshal(p *codec.Packer)

	// Size is the number of bytes it takes to represent this [Action]. This is used to preallocate
	// memory during encoding and to charge bandwidth fees.
	Size() int
}

type AuthBatchVerifier interface {
	Add([]byte, Auth) func() error
	Done() []func() error
}

type Auth interface {
	// GetTypeID uniquely identifies each supported [Auth]. We use IDs to avoid
	// reflection.
	GetTypeID() uint8

	// ValidRange is the timestamp range (in ms) that this [Auth] is considered valid.
	//
	// -1 means no start/end
	ValidRange(Rules) (start int64, end int64)

	// MaxComputeUnits is the maximum amount of compute a given [Auth] could use. This is
	// used to determine whether the [Auth] can be included in a given block and to compute
	// the required fee to execute.
	//
	// Developers should make every effort to bound this as tightly to the actual max so that
	// users don't need to have a large balance to call an [Auth] (must prepay fee before execution).
	//
	// MaxComputeUnits should take into account [AsyncVerify], [CanDeduct], [Deduct], and [Refund]
	MaxComputeUnits(Rules) uint64

	// StateKeys is a full enumeration of all database keys that could be touched during execution
	// of an [Auth]. This is used to prefetch state and will be used to parallelize execution (making
	// an execution tree is trivial).
	//
	// All keys specified must be suffixed with the number of chunks that could ever be read from that
	// key (formatted as a big-endian uint16). This is used to automatically calculate storage usage.
	StateKeys() []string

	// AsyncVerify should perform any verification that can be run concurrently. It may not be run by the time
	// [Verify] is invoked but will be checked before a [Transaction] is considered successful.
	//
	// AsyncVerify is typically used to perform cryptographic operations.
	AsyncVerify(msg []byte) error

	// Verify performs any checks against state required to determine if [Auth] is valid.
	//
	// This could be used, for example, to determine that the public key used to sign a transaction
	// is registered as the signer for an account. This could also be used to pull a [Program] from disk.
	Verify(
		ctx context.Context,
		r Rules,
		im state.Immutable,
		action Action,
	) (computeUnits uint64, err error)

	// Payer is the owner of [Auth]. It is used by the mempool to ensure that there aren't too many transactions
	// from a single Payer.
	Payer() []byte

	// CanDeduct returns an error if [amount] cannot be paid by [Auth].
	CanDeduct(ctx context.Context, im state.Immutable, amount uint64) error

	// Deduct removes [amount] from [Auth] during transaction execution to pay fees.
	Deduct(ctx context.Context, mu state.Mutable, amount uint64) error

	// Refund returns [amount] to [Auth] after transaction execution if any fees were
	// not used.
	//
	// Refund will return an error if it attempts to create any new keys. It can only
	// modify or remove existing keys.
	//
	// Refund is only invoked if [amount] > 0.
	Refund(ctx context.Context, mu state.Mutable, amount uint64) error

	// Marshal encodes an [Auth] as bytes.
	Marshal(p *codec.Packer)

	// Size is the number of bytes it takes to represent this [Auth]. This is used to preallocate
	// memory during encoding and to charge bandwidth fees.
	Size() int
}

type AuthFactory interface {
	// Sign is used by helpers, auth object should store internally to be ready for marshaling
	Sign(msg []byte, action Action) (Auth, error)

	MaxUnits() (bandwidth uint64, compute uint64, stateKeysCount []uint16)
}

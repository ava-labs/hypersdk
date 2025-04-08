// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/state"
)

type Parser interface {
	// Parse marshaled action
	ParseAction([]byte) (Action, error)
	// Parse marshaled auth
	ParseAuth([]byte) (Auth, error)
}

type Mempool interface {
	// Number of items
	Len(context.Context) int
	// Size of mempool in bytes
	Size(context.Context) int // bytes
	// Add a tx to the mempool
	Add(context.Context, []*Transaction)

	// Signal to mempool to prepare new stream of txs
	StartStreaming(context.Context)
	// Signal to mempool to prepare new stream of count txs
	PrepareStream(ctx context.Context, count int)
	// Get txs from mempool
	Stream(context.Context, int) []*Transaction
	// Restore txs to mempool and clear streamed items
	FinishStreaming(ctx context.Context, txs []*Transaction) int
}

type Genesis interface {
	// Initialize state of the chain, which may including balance allocations
	InitializeState(ctx context.Context, tracer trace.Tracer, mu state.Mutable, balanceHandler BalanceHandler) error
}

// TODO: add fixed rules as a subset of this interface
type Rules interface {
	// Should almost always be constant (unless there is a fork of
	// a live network)
	//
	// network ID
	GetNetworkID() uint32
	// chain ID
	GetChainID() ids.ID

	// minimum gap between non-empty blocks (in milliseconds)
	GetMinBlockGap() int64
	// minimum gap between empty blocks (in milliseconds)
	GetMinEmptyBlockGap() int64
	// validity window for txs (in milliseconds)
	GetValidityWindow() int64

	// maximum number of actions per transaction
	GetMaxActionsPerTx() uint8

	// minimum unit price
	GetMinUnitPrice() fees.Dimensions
	// unit price change denominator
	GetUnitPriceChangeDenominator() fees.Dimensions
	// amount of units that block consumption should target
	GetWindowTargetUnits() fees.Dimensions
	// maximum amount of units that a block can consume
	GetMaxBlockUnits() fees.Dimensions
	// minimum amount of compute
	GetBaseComputeUnits() uint64

	// Invariants:
	// * VMs must manage the max key length and max value length (max network
	//   limit is ~2MB)
	// * Creating a new key involves first allocating and then writing
	// * Keys are only charged once per transaction (even if used multiple times), it is
	//   up to the controller to ensure multiple usage has some compute cost

	// Maximum number of chunks associated with the sponsor account
	GetSponsorStateKeysMaxChunks() []uint16
	// Fixed cost of reading a kv-pair
	GetStorageKeyReadUnits() uint64
	// Variable cost of reading a kv-pair (per chunk)
	GetStorageValueReadUnits() uint64
	// Fixed cost of allocating a kv-pair
	GetStorageKeyAllocateUnits() uint64
	// Variable cost of allocating a kv-pair (per chunk)
	GetStorageValueAllocateUnits() uint64
	// Fixed cost of writing a kv-pair
	GetStorageKeyWriteUnits() uint64
	// Variable cost of writing a kv-pair (per chunk)
	GetStorageValueWriteUnits() uint64

	// Fetch a custom rule
	FetchCustom(string) (any, bool)
}

type RuleFactory interface {
	// Get rules at timestamp t
	GetRules(t int64) Rules
}

type MetadataManager interface {
	// Get prefix key for storing block height
	HeightPrefix() []byte
	// Get prefix key for storing block timestamp
	TimestampPrefix() []byte
	// Get prefix key for storing fees
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

type Action interface {
	// Unique identifier for this action.
	GetTypeID() uint8
	// ValidRange is the timestamp range (in ms) that this [Action] is considered valid.
	//
	// -1 means no start/end
	ValidRange(Rules) (start int64, end int64)

	// Bytes returns the byte representation of this action.
	// The chain parser must be able to parse this representation and return the corresponding action.
	// This function is not performance critical because actions/auth are always deserialized into
	// a transaction.
	// Transaction cache their byte representations during unmarshal, so Bytes is only called on the
	// write path ie. constructing/issuing transactions.
	//
	// The write path is not performance critical because this only impacts transaction issuers and testing.
	Bytes() []byte

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
	// If any key is removed and then re-created, this will count as a creation
	// instead of a modification.
	//
	// [actionID] is a unique, but nonrandom identifier for each [Action].
	StateKeys(actor codec.Address, actionID ids.ID) state.Keys

	// Execute actually runs the [Action]. Any state changes that the [Action] performs should
	// be done here.
	//
	// If any keys are touched during [Execute] that are not specified in [StateKeys], the transaction
	// will revert and the max fee will be charged.
	//
	// If [Execute] returns an error, execution will halt and any state changes
	// will revert.
	//
	// [actionID] is a unique, but nonrandom identifier for each [Action].
	Execute(
		ctx context.Context,
		r Rules,
		mu state.Mutable,
		timestamp int64,
		actor codec.Address,
		actionID ids.ID,
	) ([]byte, error)
}

type Auth interface {
	// GetTypeID returns the typeID of this auth instance.
	GetTypeID() uint8
	// ValidRange is the timestamp range (in ms) that this [Action] is considered valid.
	//
	// -1 means no start/end
	ValidRange(Rules) (start int64, end int64)

	// Bytes returns the byte representation of this auth credential.
	// The chain parser must be able to parse this representation and return the corresponding Auth.
	// This function is not performance critical because actions/auth are always deserialized into
	// a transaction.
	// Transaction cache their byte representations during unmarshal, so Bytes is only called on the
	// write path ie. constructing/issuing transactions.
	//
	// The write path is not performance critical because this only impacts transaction issuers and testing.
	Bytes() []byte

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
	// Add a signature to batch verification.
	// Returns a function if there's enough signatures to batch verify.
	Add([]byte, Auth) func() error
	// Returns a function to verify all outstanding signatures.
	Done() []func() error
}

type AuthFactory interface {
	// Sign is used by helpers, auth object should store internally to be ready for marshaling
	Sign(msg []byte) (Auth, error)
	// Max bandwidth and compute units that this factory consumes
	MaxUnits() (bandwidth uint64, compute uint64)
	// Address of the factory
	Address() codec.Address
}

type ValidityWindow interface {
	// Verify that there are no repeats within blk and within the validity window.
	VerifyExpiryReplayProtection(
		ctx context.Context,
		blk validitywindow.ExecutionBlock[*Transaction],
	) error
	// Accept blk as most recent
	Accept(blk validitywindow.ExecutionBlock[*Transaction])
	// Check if txs are repeats within the validity window
	IsRepeat(
		ctx context.Context,
		parentBlk validitywindow.ExecutionBlock[*Transaction],
		currentTimestamp int64,
		txs []*Transaction,
	) (set.Bits, error)
}

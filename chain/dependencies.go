// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/executor"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/vilmo"
)

type (
	ActionRegistry *codec.TypeParser[Action, *warp.Message, bool]
	AuthRegistry   *codec.TypeParser[Auth, *warp.Message, bool]
)

type Parser interface {
	Rules(int64) Rules
	Registry() (ActionRegistry, AuthRegistry)
}

type Metrics interface {
	RecordRPCAuthorizedTx()
	RecordExecutedChunks(int)

	RecordWaitRepeat(time.Duration)
	RecordWaitQueue(time.Duration)
	RecordWaitAuth(time.Duration)
	RecordWaitPrecheck(time.Duration)
	RecordWaitExec(time.Duration)
	RecordWaitCommit(time.Duration)

	RecordRemainingMempool(int)

	RecordBlockVerifyFail()
	RecordBlockVerify(time.Duration)
	RecordBlockAccept(time.Duration)
	RecordAcceptedEpoch(uint64)
	RecordExecutedEpoch(uint64)

	GetExecutorRecorder() executor.Metrics
	RecordBlockExecute(time.Duration)
	RecordTxsIncluded(int)
	RecordChunkBuildTxDropped()
	RecordBlockBuildCertDropped()
	RecordTxsInvalid(int)
	RecordEngineBacklog(int)

	RecordStateChanges(int)

	// TODO: make each name a string and then
	// allow dynamic registering of metrics
	// as needed rather than this approach (just
	// have gauge, counter, averager).
	RecordVilmoBatchInit(time.Duration)
	RecordVilmoBatchInitBytes(int64)
	RecordVilmoBatchesRewritten()
	RecordVilmoBatchPrepare(time.Duration)
	RecordTStateIterate(time.Duration)
	RecordVilmoBatchWrite(time.Duration)
}

type Monitoring interface {
	Tracer() trace.Tracer
	Logger() logging.Logger
}

type VM interface {
	Metrics
	Monitoring
	Parser

	// TODO: cleanup
	Engine() *Engine
	RequestChunks(uint64, []*ChunkCertificate, chan *Chunk)
	SubnetID() ids.ID

	IsBootstrapped() bool
	LastAcceptedBlock() *StatelessBlock
	GetStatelessBlock(context.Context, ids.ID) (*StatelessBlock, error)

	State() *vilmo.Vilmo
	StateManager() StateManager
	ValidatorState() validators.State

	IsIssuedTx(context.Context, *Transaction) bool
	IssueTx(context.Context, *Transaction)

	GetAuthResult(ids.ID) bool
	IsRepeatTx(context.Context, []*Transaction, set.Bits) set.Bits
	IsRepeatChunk(context.Context, []*ChunkCertificate, set.Bits) set.Bits

	Mempool() Mempool
	GetTargetChunkBuildDuration() time.Duration
	GetPrecheckCores() int
	GetActionExecutionCores() int

	Verified(context.Context, *StatelessBlock)
	Rejected(context.Context, *StatelessBlock)
	Accepted(context.Context, *StatelessBlock, []*FilteredChunk)
	ExecutedChunk(context.Context, *StatefulBlock, *FilteredChunk, []*Result, []ids.ID)
	ExecutedBlock(context.Context, *StatefulBlock)

	// TODO: cleanup
	NodeID() ids.NodeID
	Signer() *bls.PublicKey
	Beneficiary() codec.Address

	Sign(*warp.UnsignedMessage) ([]byte, error)
	StopChan() chan struct{}

	StartCertStream(context.Context)
	StreamCert(context.Context) (*ChunkCertificate, bool)
	FinishCertStream(context.Context, []*ChunkCertificate)
	HasChunk(ctx context.Context, slot int64, id ids.ID) bool
	RestoreChunkCertificates(context.Context, []*ChunkCertificate)
	IsSeenChunk(context.Context, ids.ID) bool
	GetChunk(int64, ids.ID) (*Chunk, error)

	IsValidHeight(ctx context.Context, height uint64) (bool, error)
	CacheValidators(ctx context.Context, height uint64)
	IsValidator(ctx context.Context, height uint64, nodeID ids.NodeID) (bool, error)                                       // TODO: filter based on being part of whole epoch
	GetAggregatePublicKey(ctx context.Context, height uint64, signers set.Bits, num, denom uint64) (*bls.PublicKey, error) // cached
	AddressPartition(ctx context.Context, epoch uint64, height uint64, addr codec.Address, partition uint8) (ids.NodeID, error)
	AddressPartitionByNamespace(ctx context.Context, epoch uint64, height uint64, ns []byte, partition uint8) (ids.NodeID, error)
}

type Mempool interface {
	Len(context.Context) int  // items
	Size(context.Context) int // bytes
	Add(context.Context, []*Transaction)

	StartStreaming(context.Context)
	Stream(context.Context) (*Transaction, bool)
	FinishStreaming(context.Context, []*Transaction)
}

type Rules interface {
	// Should almost always be constant (unless there is a fork of
	// a live network)
	NetworkID() uint32
	ChainID() ids.ID

	// TODO: make immutable rules (that don't expect to be changed)
	GetPartitions() uint8
	GetBlockExecutionDepth() uint64
	GetEpochDuration() int64

	GetMinBlockGap() int64    // in milliseconds
	GetValidityWindow() int64 // in milliseconds

	GetUnitPrices() Dimensions // TODO: make this dynamic if we want to burn fees?
	GetMaxChunkUnits() Dimensions

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
	GetSponsorStateKeyChunks() []uint16
	GetStorageKeyReadUnits() uint64
	GetStorageValueReadUnits() uint64 // per chunk
	GetStorageKeyAllocateUnits() uint64
	GetStorageValueAllocateUnits() uint64 // per chunk
	GetStorageKeyWriteUnits() uint64
	GetStorageValueWriteUnits() uint64 // per chunk

	GetWarpConfig(sourceChainID ids.ID) (bool, uint64, uint64)

	FetchCustom(string) (any, bool)
}

type MetadataManager interface {
	HeightKey() string
	PHeightKey() string
	TimestampKey() string
}

type WarpManager interface {
	IncomingWarpKeyPrefix(sourceChainID ids.ID, msgID ids.ID) string
	OutgoingWarpKeyPrefix(txID ids.ID) string
}

type FeeHandler interface {
	// StateKeys is a full enumeration of all database keys that could be touched during fee payment
	// by [addr] or during bond check/claim.
	//
	// This is used to prefetch state and will be used to parallelize execution (making
	// an execution tree is trivial).
	//
	// All keys specified must be suffixed with the number of chunks that could ever be read from that
	// key (formatted as a big-endian uint16). This is used to automatically calculate storage usage.
	SponsorStateKeys(addr codec.Address) state.Keys

	// CanDeduct returns an error if [amount] cannot be paid by [addr].
	CanDeduct(ctx context.Context, addr codec.Address, im state.Immutable, amount uint64) (bool, error)

	// Deduct removes [amount] from [addr] during transaction execution to pay fees.
	Deduct(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64) error

	// IsFrozen returns true if transactions from [addr] are not allowed to be submitted.
	// IsFrozen(ctx context.Context, addr codec.Address, epoch uint64, im state.Immutable) (bool, error)    // account can submit
	// IsClaimed(ctx context.Context, addr codec.Address, epoch uint64, im state.Immutable) (bool, error)   // some bond is claimed
	// EpochBond(ctx context.Context, addr codec.Address, epoch uint64, im state.Immutable) (uint64, error) // total locked is this value * 2
	// ClaimBond(ctx context.Context, addr codec.Address, epoch uint64, mu state.Mutable) error             // Must handle after execution to avoid conflicts, if already claimed, does nothing
	// TODO: can't attempt to unfreeze until latest claim key + 2 (to give time for all claims to be processed) and/or until a new bond takes effect claims:<[epoch][epoch]> balance:<[balance][bond][epoch][new bond]>
	//  when unfrozen, we delete the claim key and then set [bond]=0 and [epoch][new bond]
	//  TODO: claims handled in random order, we need to handle deterministically to get canonical epoch/epoch result
}

type EpochManager interface {
	// EpochKey is the key that corresponds to the height of the P-Chain to use for
	// validation of a given epoch and the fees to use for verifying transactions.
	EpochKey(epoch uint64) string
}

type RewardHandler interface {
	// Reward sends [amount] to [addr] after block execution if any fees or bonds were collected.
	//
	// Reward is only invoked if [amount] > 0.
	// Reward(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64) error
}

// StateManager allows [Chain] to safely store certain types of items in state
// in a structured manner. If we did not use [StateManager], we may overwrite
// state written by actions or auth.
//
// None of these keys should be suffixed with the max amount of chunks they will
// use. This will be handled by the hypersdk.
type StateManager interface {
	MetadataManager
	WarpManager
	FeeHandler
	EpochManager
	RewardHandler
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

	// ComputeUnits is the amount of compute an [Action] uses. This is used to determine whether the
	// [Action] can be included in a given block and to compute the required fee to execute.
	ComputeUnits(Rules) uint64

	// StateKeyChunks is used to estimate the fee a transaction should pay. It includes the chunks each
	// state key could use without requiring the state keys to actually be provided (may not be known
	// until execution).
	StateKeyChunks() []uint16

	// StateKeys is a full enumeration of all database keys that could be touched during execution
	// of an [Action]. This is used to prefetch state and will be used to parallelize execution (making
	// an execution tree is trivial).
	//
	// All keys specified must be suffixed with the number of chunks that could ever be read from that
	// key (formatted as a big-endian uint16). This is used to automatically calculate storage usage.
	//
	// If any key is removed and then re-created, this will count as a creation instead of a modification.
	StateKeys(actor codec.Address, txID ids.ID) state.Keys

	// Execute actually runs the [Action]. Any state changes that the [Action] performs should
	// be done here.
	//
	// If any keys are touched during [Execute] that are not specified in [StateKeys], the transaction
	// will revert.
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
	) (success bool, output []byte, warpMessage *warp.UnsignedMessage, err error)

	// OutputsWarpMessage indicates whether an [Action] will produce a warp message. The max size
	// of any warp message is [MaxOutgoingWarpChunks].
	OutputsWarpMessage() bool

	NMTNamespace() []byte
}

type Auth interface {
	Object

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
}

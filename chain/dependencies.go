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
	Len(context.Context) int
	Add(context.Context, []*Transaction)

	Top(
		context.Context,
		time.Duration,
		func(context.Context, *Transaction) (cont bool, restore bool, err error),
	) error

	StartStreaming(context.Context)
	PrepareStream(context.Context, int)
	Stream(context.Context, int) []*Transaction
	FinishStreaming(context.Context, []*Transaction) int
}

type Database interface {
	GetValue(ctx context.Context, key []byte) ([]byte, error)
	Insert(ctx context.Context, key []byte, value []byte) error
	Remove(ctx context.Context, key []byte) error
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

	GetMaxKeySize() uint32
	GetMaxValueSize() uint16 // in chunks

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
type StateManager interface {
	HeightKey() []byte
	FeeKey() []byte
	IncomingWarpKey(sourceChainID ids.ID, msgID ids.ID) []byte
	OutgoingWarpKey(txID ids.ID) []byte
}

type Action interface {
	GetTypeID() uint8                          // identify uniquely the action
	ValidRange(Rules) (start int64, end int64) // -1 means no start/end

	// MaxUnits is used to determine whether the [Action] is executable in the given block. Implementors
	// should make a best effort to specify these as close to possible to the actual max.
	MaxComputeUnits(Rules) uint64

	// Used to determine how many fees to charge for storage
	OutputsWarpMessage() bool

	// Auth may contain an [Actor] that performs a transaction
	//
	// We provide the [txID] here because different actions like to use this as
	// a unique identifier for things created in an action.
	//
	// If attempt to reference missing key, error...it is ok to not use all keys (conditional logic based on state)
	//
	// Always assume the last 4 bytes are a uint32 of the max size to read
	StateKeys(auth Auth, txID ids.ID) []string

	// StateKeysCount is used for fee estimation when actual state keys can't be generated
	StateKeysCount() []uint16

	// Key distinction with "Auth" is the payment of fees. All non-fee payments
	// occur in Execute but Auth handles fees.
	//
	// The weird part of this is that they both need a shared understanding of
	// balance tracking. Is it weird Auth then needs an understanding of storage
	// structure? Not sure there is an easier way.
	//
	// It is also odd because we may pull some aspect of the transaction from
	// auth (like where to pull balance from on a transfer).
	Execute(
		ctx context.Context,
		r Rules,
		db Database,
		timestamp int64,
		auth Auth,
		txID ids.ID,
		warpVerified bool,
	) (success bool, computeUnits uint64, output []byte, warpMessage *warp.UnsignedMessage, err error) // err should only be returned if fatal

	Marshal(p *codec.Packer)
	Size() int
}

type AuthBatchVerifier interface {
	Add([]byte, Auth) func() error
	Done() []func() error
}

type Auth interface {
	GetTypeID() uint8                          // identify uniquely the auth
	ValidRange(Rules) (start int64, end int64) // -1 means no start/end

	// MaxComputeUnits should take into account [AsyncVerify], [CanDeduct], [Deduct], and [Refund]
	MaxComputeUnits(Rules) uint64

	StateKeys() []string

	// AsyncVerify will be run concurrently, optimistically start crypto ops (may not complete before [Verify])
	AsyncVerify(msg []byte) error

	// ComputeUnits should take into account [AsyncVerify], [CanDeduct], [Deduct], and [Refund]
	Verify(
		ctx context.Context,
		r Rules,
		db Database, // Should only read, no mutate
		action Action, // Authentication may be scoped to action type
	) (computeUnits uint64, err error) // if there is account abstraction, may need to pull from state some mapping

	Payer() []byte // need to track mempool + charge fees -> used to clear related accounts if balance check fails

	CanDeduct(ctx context.Context, db Database, amount uint64) error
	Deduct(ctx context.Context, db Database, amount uint64) error

	Refund(ctx context.Context, db Database, amount uint64) error // only invoked if amount > 0

	Marshal(p *codec.Packer)
	Size() int
}

type AuthFactory interface {
	// Sign is used by helpers, auth object should store internally to be ready for marshaling
	Sign(msg []byte, action Action) (Auth, error)

	MaxUnits() (bandwidth uint64, compute uint64, stateKeysCount []uint16)
}

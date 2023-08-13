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

	Workers() *workers.Workers
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
	RecordStateChanges(int)
	RecordStateOperations(int)
	RecordBuildCapped()
	RecordEmptyBlockBuilt()
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

	GetMinBlockGap() int64
	GetMinEmptyBlockGap() int64

	GetMinUnitPrice() uint64
	GetUnitPriceChangeDenominator() uint64
	GetWindowTargetUnits() uint64
	GetMaxBlockUnits() uint64 // should ensure can't get above block max size

	GetBaseUnits() uint64
	GetWarpBaseUnits() uint64
	GetWarpUnitsPerSigner() uint64

	GetWarpConfig(sourceChainID ids.ID) (bool, uint64, uint64)

	GetValidityWindow() int64 // in milliseconds

	FetchCustom(string) (any, bool)
}

// StateManager allows [Chain] to safely store certain types of items in state
// in a structured manner. If we did not use [StateManager], we may overwrite
// state written by actions or auth.
type StateManager interface {
	HeightKey() []byte
	IncomingWarpKey(sourceChainID ids.ID, msgID ids.ID) []byte
	OutgoingWarpKey(txID ids.ID) []byte
}

type Action interface {
	GetTypeID() uint8                          // identify uniquely the action
	MaxUnits(Rules) uint64                     // max units that could be charged via execute
	ValidRange(Rules) (start int64, end int64) // -1 means no start/end

	// Auth may contain an [Actor] that performs a transaction
	//
	// We provide the [txID] here because different actions like to use this as
	// a unique identifier for things created in an action.
	//
	// If attempt to reference missing key, error...it is ok to not use all keys (conditional logic based on state)
	StateKeys(auth Auth, txID ids.ID) [][]byte

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
	) (result *Result, err error) // err should only be returned if fatal

	Size() int
	Marshal(p *codec.Packer)
}

type AuthBatchVerifier interface {
	Add([]byte, Auth) func() error
	Done() []func() error
}

type Auth interface {
	GetTypeID() uint8 // identify uniquely the auth

	MaxUnits(Rules) uint64
	ValidRange(Rules) (start int64, end int64) // -1 means no start/end

	StateKeys() [][]byte

	// will be run concurrently, optimistically start crypto ops (may not complete before [Verify])
	AsyncVerify(msg []byte) error

	// Is Auth able to execute [Action], assuming [AsyncVerify] passes?
	Verify(
		ctx context.Context,
		r Rules,
		db Database, // Should only read, no mutate
		action Action, // Authentication may be scoped to action type
	) (units uint64, err error) // if there is account abstraction, may need to pull from state some mapping
	// if verify is not validate, then what? -> can't actually change fee then?
	// units should include any cost associated with [AsyncVerify]

	// TODO: identifier->may be used to send to in action as well?
	Payer() []byte // need to track mempool + charge fees -> used to clear related accounts if balance check fails
	CanDeduct(ctx context.Context, db Database, amount uint64) error
	Deduct(ctx context.Context, db Database, amount uint64) error
	Refund(ctx context.Context, db Database, amount uint64) error // only invoked if amount > 0

	Size() int
	Marshal(p *codec.Packer)
}

type AuthFactory interface {
	// used by helpers, auth object should store internally to be ready for marshaling
	Sign(msg []byte, action Action) (Auth, error)
}

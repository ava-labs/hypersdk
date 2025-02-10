// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package types

import (
	"context"

	"github.com/StephenButtolph/canoto"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

type Action[T any] interface {
	canoto.FieldMaker[T]

	// ComputeUnits is the amount of compute required to call [Execute]. This is used to determine
	// whether the [Action] can be included in a given block and to compute the required fee to execute.
	ComputeUnits() uint64

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
		mu state.Mutable,
		timestamp int64,
		actor codec.Address,
		actionID ids.ID,
	) (codec.Typed, error)
}

type Auth[T any] interface {
	canoto.FieldMaker[T]

	// ComputeUnits is the amount of compute required to call [Verify]. This is
	// used to determine whether [Auth] can be included in a given block and to compute
	// the required fee to execute.
	ComputeUnits() uint64

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

type AuthFactory[A Auth[A]] interface {
	// Sign is used by helpers, auth object should store internally to be ready for marshaling
	Sign(msg []byte) (A, error)
	MaxUnits() (bandwidth uint64, compute uint64)
	Address() codec.Address
}

type TransactionData[T Action[T]] struct {
	// Expiry is the Unix time (in milliseconds) that this transaction expires.
	// Once a block with a timestamp past [Expiry] is accepted, this transaction has either
	// been included or expired, which means that it can be re-issued without fear of replay.
	Expiry int64 `canoto:"sint,1" json:"expiry"`
	// ChainID provides replay protection for transactions that may otherwise be identical
	// on multiple chains.
	ChainID ids.ID `canoto:"fixed bytes,2" json:"chainID"`
	// MaxFee specifies the maximum fee that the transaction is willing to pay. The chain
	// will charge up to this amount if the transaction is included in a block.
	// If this fee is too low to cover all fees, the transaction may be dropped from the mempool.
	MaxFee uint64 `canoto:"fint64,3" json:"maxFee"`
	// Actions are the operations that this transaction will perform if included and executed
	// onchain. If any action fails, all actions will be reverted.
	Actions []T `canoto:"repeated field,4" json:"actions"`

	unsignedBytes []byte

	canotoData canotoData_TransactionData
}

func NewTransactionData[T Action[T]](
	expiry int64,
	chainID ids.ID,
	maxFee uint64,
	actions []T,
) *TransactionData[T] {
	return &TransactionData[T]{
		Expiry:  expiry,
		ChainID: chainID,
		MaxFee:  maxFee,
		Actions: actions,
	}
}

func (t *TransactionData[T]) init() {
	if len(t.unsignedBytes) != 0 {
		return
	}

	t.unsignedBytes = t.MarshalCanoto()
}

func (t *TransactionData[T]) UnsignedBytes() []byte {
	t.init()
	return t.unsignedBytes
}

func (t *TransactionData[T]) GetExpiry() int64 { return t.Expiry }

func SignTx[T Action[T], A Auth[A]](txData *TransactionData[T], authFactory AuthFactory[A]) (*Transaction[T, A], error) {
	auth, err := authFactory.Sign(txData.UnsignedBytes())
	if err != nil {
		return nil, err
	}
	return NewTransaction(txData, auth), nil
}

type Transaction[T Action[T], A Auth[A]] struct {
	*TransactionData[T] `canoto:"field,1" json:"unsignedTx"`

	Auth A `canoto:"field,2" json:"signature"`

	bytes      []byte
	txID       ids.ID
	canotoData canotoData_Transaction
}

func NewTransaction[T Action[T], A Auth[A]](
	transactionData *TransactionData[T],
	auth A,
) *Transaction[T, A] {
	tx := &Transaction[T, A]{
		TransactionData: transactionData,
		Auth:            auth,
	}
	tx.init()
	return tx
}

func (t *Transaction[T, A]) init() {
	if len(t.bytes) != 0 {
		return
	}

	t.bytes = t.MarshalCanoto()
	t.txID = utils.ToID(t.bytes)
}

func (t *Transaction[T, A]) GetID() ids.ID {
	t.init()
	return t.txID
}

func (t *Transaction[T, A]) GetBytes() []byte {
	t.init()
	return t.bytes
}

func ParseTransaction[T Action[T], A Auth[A]](bytes []byte) (*Transaction[T, A], error) {
	tx := &Transaction[T, A]{}
	if err := tx.UnmarshalCanoto(bytes); err != nil {
		return nil, err
	}
	tx.bytes = bytes
	tx.txID = utils.ToID(bytes)
	return tx, nil
}

func (t *Transaction[T, A]) GetSponsor() codec.Address {
	return t.Auth.Sponsor()
}

func (t *Transaction[T, A]) VerifyAuth(ctx context.Context) error {
	return t.Auth.Verify(ctx, t.UnsignedBytes())
}

func (t *Transaction[T, A]) GetChainID() ids.ID {
	return t.ChainID
}
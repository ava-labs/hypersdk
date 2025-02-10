// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"fmt"

	"github.com/StephenButtolph/canoto"
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/math"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
	"github.com/ava-labs/hypersdk/utils"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
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
	stateKeys  state.Keys
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

func (t *Transaction[T, A]) GetSize() int {
	t.init()
	return len(t.bytes)
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

func (t *Transaction[T, A]) StateKeys(bh BalanceHandler) (state.Keys, error) {
	if t.stateKeys != nil {
		return t.stateKeys, nil
	}
	stateKeys := make(state.Keys)

	// Verify the formatting of state keys passed by the controller
	for i, action := range t.Actions {
		for k, v := range action.StateKeys(t.Auth.Actor(), CreateActionID(t.GetID(), uint8(i))) {
			if !stateKeys.Add(k, v) {
				return nil, ErrInvalidKeyValue
			}
		}
	}
	for k, v := range bh.SponsorStateKeys(t.Auth.Sponsor()) {
		if !stateKeys.Add(k, v) {
			return nil, ErrInvalidKeyValue
		}
	}

	// Cache keys if called again
	t.stateKeys = stateKeys
	return stateKeys, nil
}

// Units is charged whether or not a transaction is successful.
func (t *Transaction[T, A]) Units(bh BalanceHandler, r Rules) (fees.Dimensions, error) {
	// Calculate compute usage
	computeOp := math.NewUint64Operator(r.GetBaseComputeUnits())
	for _, action := range t.Actions {
		computeOp.Add(action.ComputeUnits())
	}
	computeOp.Add(t.Auth.ComputeUnits())
	maxComputeUnits, err := computeOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}

	// Calculate storage usage
	stateKeys, err := t.StateKeys(bh)
	if err != nil {
		return fees.Dimensions{}, err
	}
	readsOp := math.NewUint64Operator(0)
	allocatesOp := math.NewUint64Operator(0)
	writesOp := math.NewUint64Operator(0)
	for k := range stateKeys {
		// Compute key costs
		readsOp.Add(r.GetStorageKeyReadUnits())
		allocatesOp.Add(r.GetStorageKeyAllocateUnits())
		writesOp.Add(r.GetStorageKeyWriteUnits())

		// Compute value costs
		maxChunks, ok := keys.MaxChunks([]byte(k))
		if !ok {
			return fees.Dimensions{}, ErrInvalidKeyValue
		}
		readsOp.MulAdd(uint64(maxChunks), r.GetStorageValueReadUnits())
		allocatesOp.MulAdd(uint64(maxChunks), r.GetStorageValueAllocateUnits())
		writesOp.MulAdd(uint64(maxChunks), r.GetStorageValueWriteUnits())
	}
	reads, err := readsOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}
	allocates, err := allocatesOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}
	writes, err := writesOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}
	return fees.Dimensions{uint64(t.GetSize()), maxComputeUnits, reads, allocates, writes}, nil
}

func (t *Transaction[T, A]) VerifyMetadata(r Rules, timestamp int64) error {
	if t.ChainID != r.GetChainID() {
		return fmt.Errorf("%w: chainID=%s, expected=%s", ErrInvalidChainID, t.ChainID, r.GetChainID())
	}
	return validitywindow.VerifyTimestamp(t.Expiry, timestamp, validityWindowTimestampDivisor, r.GetValidityWindow())
}

func (t *Transaction[T, A]) PreExecute(
	ctx context.Context,
	feeManager *internalfees.Manager,
	bh BalanceHandler,
	r Rules,
	im state.Immutable,
	timestamp int64,
) error {
	if err := t.VerifyMetadata(r, timestamp); err != nil {
		return err
	}
	if len(t.Actions) > int(r.GetMaxActionsPerTx()) {
		return ErrTooManyActions
	}
	units, err := t.Units(bh, r)
	if err != nil {
		return err
	}
	fee, err := feeManager.Fee(units)
	if err != nil {
		return err
	}
	return bh.CanDeduct(ctx, t.Auth.Sponsor(), im, fee)
}

// Execute after knowing a transaction can pay a fee. Attempt
// to charge the fee in as many cases as possible.
//
// Invariant: [PreExecute] is called just before [Execute]
func (t *Transaction[T, A]) Execute(
	ctx context.Context,
	feeManager *internalfees.Manager,
	bh BalanceHandler,
	r Rules,
	ts *tstate.TStateView,
	timestamp int64,
) (*Result, error) {
	// Always charge fee first
	units, err := t.Units(bh, r)
	if err != nil {
		// Should never happen
		return nil, fmt.Errorf("failed to calculate tx units: %w", err)
	}
	fee, err := feeManager.Fee(units)
	if err != nil {
		// Should never happen
		return nil, fmt.Errorf("failed to calculate tx fee: %w", err)
	}
	if err := bh.Deduct(ctx, t.Auth.Sponsor(), ts, fee); err != nil {
		// This should never fail for low balance (as we check [CanDeductFee]
		// immediately before).
		return nil, fmt.Errorf("failed to deduct tx fee: %w", err)
	}

	// We create a temp state checkpoint to ensure we don't commit failed actions to state.
	//
	// We should favor reverting over returning an error because the caller won't be charged
	// for a transaction that returns an error.
	var (
		actionStart   = ts.OpIndex()
		actionOutputs = [][]byte{}
	)
	for i, action := range t.Actions {
		actionOutput, err := action.Execute(ctx, r, ts, timestamp, t.Auth.Actor(), CreateActionID(t.GetID(), uint8(i)))
		if err != nil {
			ts.Rollback(ctx, actionStart)
			return &Result{false, utils.ErrBytes(err), actionOutputs, units, fee}, nil
		}

		var encodedOutput []byte
		if actionOutput == nil {
			// Ensure output standardization (match form we will
			// unmarshal)
			encodedOutput = []byte{}
		} else {
			encodedOutput, err = MarshalTyped(actionOutput)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal action output %T: %w", actionOutput, err)
			}
		}

		actionOutputs = append(actionOutputs, encodedOutput)
	}
	return &Result{
		Success: true,
		Error:   []byte{},

		Outputs: actionOutputs,

		Units: units,
		Fee:   fee,
	}, nil
}

// EstimateUnits provides a pessimistic estimate (some key accesses may be duplicates) of the cost
// to execute a transaction.
//
// This is typically used during transaction construction.
func EstimateUnits(r Rules, actions Actions, authFactory AuthFactory) (fees.Dimensions, error) {
	var (
		bandwidth          = uint64(BaseSize)
		stateKeysMaxChunks = []uint16{} // TODO: preallocate
		computeOp          = math.NewUint64Operator(r.GetBaseComputeUnits())
		readsOp            = math.NewUint64Operator(0)
		allocatesOp        = math.NewUint64Operator(0)
		writesOp           = math.NewUint64Operator(0)
	)

	// Calculate over action/auth
	bandwidth += consts.Uint8Len
	for i, action := range actions {
		actionSize, err := GetSize(action)
		if err != nil {
			return fees.Dimensions{}, err
		}

		actor := authFactory.Address()
		stateKeys := action.StateKeys(actor, CreateActionID(ids.Empty, uint8(i)))
		actionStateKeysMaxChunks, ok := stateKeys.ChunkSizes()
		if !ok {
			return fees.Dimensions{}, ErrInvalidKeyValue
		}
		bandwidth += consts.ByteLen + uint64(actionSize)
		stateKeysMaxChunks = append(stateKeysMaxChunks, actionStateKeysMaxChunks...)
		computeOp.Add(action.ComputeUnits(r))
	}
	authBandwidth, authCompute := authFactory.MaxUnits()
	bandwidth += consts.ByteLen + authBandwidth
	sponsorStateKeyMaxChunks := r.GetSponsorStateKeysMaxChunks()
	stateKeysMaxChunks = append(stateKeysMaxChunks, sponsorStateKeyMaxChunks...)
	computeOp.Add(authCompute)

	// Estimate compute costs
	compute, err := computeOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}

	// Estimate storage costs
	for _, maxChunks := range stateKeysMaxChunks {
		// Compute key costs
		readsOp.Add(r.GetStorageKeyReadUnits())
		allocatesOp.Add(r.GetStorageKeyAllocateUnits())
		writesOp.Add(r.GetStorageKeyWriteUnits())

		// Compute value costs
		readsOp.MulAdd(uint64(maxChunks), r.GetStorageValueReadUnits())
		allocatesOp.MulAdd(uint64(maxChunks), r.GetStorageValueAllocateUnits())
		writesOp.MulAdd(uint64(maxChunks), r.GetStorageValueWriteUnits())
	}
	reads, err := readsOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}
	allocates, err := allocatesOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}
	writes, err := writesOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}
	return fees.Dimensions{bandwidth, compute, reads, allocates, writes}, nil
}

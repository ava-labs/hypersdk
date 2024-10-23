// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/emap"
	"github.com/ava-labs/hypersdk/internal/math"
	"github.com/ava-labs/hypersdk/internal/mempool"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
	"github.com/ava-labs/hypersdk/utils"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

var (
	_ emap.Item    = (*Transaction)(nil)
	_ mempool.Item = (*Transaction)(nil)
)

type TransactionData struct {
	Base *Base `json:"base"`

	Actions Actions `json:"actions"`

	unsignedBytes []byte
}

func NewTxData(base *Base, actions Actions) *TransactionData {
	return &TransactionData{
		Base:    base,
		Actions: actions,
	}
}

// UnsignedBytes returns the byte slice representation of the tx
func (t *TransactionData) UnsignedBytes() ([]byte, error) {
	if len(t.unsignedBytes) > 0 {
		return t.unsignedBytes, nil
	}
	size := t.Base.Size() + consts.Uint8Len

	actionsSize, err := t.Actions.Size()
	if err != nil {
		return nil, err
	}
	size += actionsSize

	p := codec.NewWriter(size, consts.NetworkSizeLimit)
	if err := t.marshal(p); err != nil {
		return nil, err
	}
	t.unsignedBytes = p.Bytes()
	return t.unsignedBytes, p.Err()
}

// Sign returns a new signed transaction with the unsigned tx copied from
// the original and a signature provided by the authFactory
func (t *TransactionData) Sign(
	factory AuthFactory,
	actionCodec *codec.TypeParser[Action],
	authCodec *codec.TypeParser[Auth],
) (*Transaction, error) {
	msg, err := t.UnsignedBytes()
	if err != nil {
		return nil, err
	}
	auth, err := factory.Sign(msg)
	if err != nil {
		return nil, err
	}

	signedTransaction := Transaction{
		TransactionData: TransactionData{
			Base:    t.Base,
			Actions: t.Actions,
		},
		Auth: auth,
	}

	// Ensure transaction is fully initialized and correct by reloading it from
	// bytes
	size := len(msg) + consts.ByteLen + auth.Size()
	p := codec.NewWriter(size, consts.NetworkSizeLimit)
	if err := signedTransaction.Marshal(p); err != nil {
		return nil, err
	}
	if err := p.Err(); err != nil {
		return nil, err
	}
	p = codec.NewReader(p.Bytes(), consts.MaxInt)
	return UnmarshalTx(p, actionCodec, authCodec)
}

func (t *TransactionData) Expiry() int64 { return t.Base.Timestamp }

func (t *TransactionData) MaxFee() uint64 { return t.Base.MaxFee }

func (t *TransactionData) Marshal(p *codec.Packer) error {
	if len(t.unsignedBytes) > 0 {
		p.PackFixedBytes(t.unsignedBytes)
		return p.Err()
	}
	return t.marshal(p)
}

func (t *TransactionData) marshal(p *codec.Packer) error {
	t.Base.Marshal(p)
	return t.Actions.marshalInto(p)
}

type Actions []Action

func (a Actions) Size() (int, error) {
	var size int
	for _, action := range a {
		actionSize, err := GetSize(action)
		if err != nil {
			return 0, err
		}
		size += consts.ByteLen + actionSize
	}
	return size, nil
}

func (a Actions) marshalInto(p *codec.Packer) error {
	p.PackByte(uint8(len(a)))
	for _, action := range a {
		p.PackByte(action.GetTypeID())
		err := marshalInto(action, p)
		if err != nil {
			return err
		}
	}
	return nil
}

type Transaction struct {
	TransactionData

	Auth Auth `json:"auth"`

	bytes     []byte
	size      int
	id        ids.ID
	stateKeys state.Keys
}

func (t *Transaction) Bytes() []byte { return t.bytes }

func (t *Transaction) Size() int { return t.size }

func (t *Transaction) ID() ids.ID { return t.id }

func (t *Transaction) StateKeys(bh BalanceHandler) (state.Keys, error) {
	if t.stateKeys != nil {
		return t.stateKeys, nil
	}
	stateKeys := make(state.Keys)

	// Verify the formatting of state keys passed by the controller
	for _, action := range t.Actions {
		for k, v := range action.StateKeys(t.Auth.Actor()) {
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
func (t *Transaction) Units(bh BalanceHandler, r Rules) (fees.Dimensions, error) {
	// Calculate compute usage
	computeOp := math.NewUint64Operator(r.GetBaseComputeUnits())
	for _, action := range t.Actions {
		computeOp.Add(action.ComputeUnits(r))
	}
	computeOp.Add(t.Auth.ComputeUnits(r))
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
	return fees.Dimensions{uint64(t.Size()), maxComputeUnits, reads, allocates, writes}, nil
}

func (t *Transaction) PreExecute(
	ctx context.Context,
	feeManager *internalfees.Manager,
	bh BalanceHandler,
	r Rules,
	im state.Immutable,
	timestamp int64,
) error {
	if err := t.Base.Execute(r, timestamp); err != nil {
		return err
	}
	if len(t.Actions) > int(r.GetMaxActionsPerTx()) {
		return ErrTooManyActions
	}
	for i, action := range t.Actions {
		start, end := action.ValidRange(r)
		if start >= 0 && timestamp < start {
			return fmt.Errorf("%w: action type %d at index %d", ErrActionNotActivated, action.GetTypeID(), i)
		}
		if end >= 0 && timestamp > end {
			return fmt.Errorf("%w: action type %d at index %d", ErrActionNotActivated, action.GetTypeID(), i)
		}
	}
	start, end := t.Auth.ValidRange(r)
	if start >= 0 && timestamp < start {
		return ErrAuthNotActivated
	}
	if end >= 0 && timestamp > end {
		return ErrAuthNotActivated
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
func (t *Transaction) Execute(
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
		return nil, err
	}
	fee, err := feeManager.Fee(units)
	if err != nil {
		// Should never happen
		return nil, err
	}
	if err := bh.Deduct(ctx, t.Auth.Sponsor(), ts, fee); err != nil {
		// This should never fail for low balance (as we check [CanDeductFee]
		// immediately before).
		return nil, err
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
		actionOutput, err := action.Execute(ctx, r, ts, timestamp, t.Auth.Actor(), CreateActionID(t.ID(), uint8(i)))
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
				return nil, err
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

// Sponsor is the [codec.Address] that pays fees for this transaction.
func (t *Transaction) Sponsor() codec.Address { return t.Auth.Sponsor() }

func (t *Transaction) Marshal(p *codec.Packer) error {
	if len(t.bytes) > 0 {
		p.PackFixedBytes(t.bytes)
		return p.Err()
	}
	return t.marshal(p)
}

func (t *Transaction) marshal(p *codec.Packer) error {
	if err := t.TransactionData.marshal(p); err != nil {
		return err
	}

	authID := t.Auth.GetTypeID()
	p.PackByte(authID)
	t.Auth.Marshal(p)

	return p.Err()
}

// VerifyAuth verifies that the transaction was signed correctly.
func (t *Transaction) VerifyAuth(ctx context.Context) error {
	msg, err := t.UnsignedBytes()
	if err != nil {
		// Should never occur because populated during unmarshal
		return err
	}
	return t.Auth.Verify(ctx, msg)
}

func UnmarshalTxData(
	p *codec.Packer,
	actionRegistry *codec.TypeParser[Action],
) (*TransactionData, error) {
	start := p.Offset()
	base, err := UnmarshalBase(p)
	if err != nil {
		return nil, fmt.Errorf("%w: could not unmarshal base", err)
	}
	actions, err := UnmarshalActions(p, actionRegistry)
	if err != nil {
		return nil, fmt.Errorf("%w: could not unmarshal actions", err)
	}

	var tx TransactionData
	tx.Base = base
	tx.Actions = actions
	if err := p.Err(); err != nil {
		return nil, p.Err()
	}
	codecBytes := p.Bytes()
	tx.unsignedBytes = codecBytes[start:p.Offset()] // ensure errors handled before grabbing memory
	return &tx, nil
}

func UnmarshalActions(
	p *codec.Packer,
	actionRegistry *codec.TypeParser[Action],
) (Actions, error) {
	actionCount := p.UnpackByte()
	if actionCount == 0 {
		return nil, fmt.Errorf("%w: no actions", ErrInvalidObject)
	}
	actions := Actions{}
	for i := uint8(0); i < actionCount; i++ {
		action, err := actionRegistry.Unmarshal(p)
		if err != nil {
			return nil, fmt.Errorf("%w: could not unmarshal action", err)
		}
		actions = append(actions, action)
	}
	return actions, nil
}

func UnmarshalTxs(
	raw []byte,
	initialCapacity int,
	actionRegistry *codec.TypeParser[Action],
	authRegistry *codec.TypeParser[Auth],
) (map[uint8]int, []*Transaction, error) {
	p := codec.NewReader(raw, consts.NetworkSizeLimit)
	txCount := p.UnpackInt(true)
	authCounts := map[uint8]int{}
	txs := make([]*Transaction, 0, initialCapacity) // DoS to set size to txCount
	for i := uint32(0); i < txCount; i++ {
		tx, err := UnmarshalTx(p, actionRegistry, authRegistry)
		if err != nil {
			return nil, nil, err
		}
		txs = append(txs, tx)
		authCounts[tx.Auth.GetTypeID()]++
	}
	if !p.Empty() {
		// Ensure no leftover bytes
		return nil, nil, ErrInvalidObject
	}
	return authCounts, txs, p.Err()
}

func UnmarshalTx(
	p *codec.Packer,
	actionRegistry *codec.TypeParser[Action],
	authRegistry *codec.TypeParser[Auth],
) (*Transaction, error) {
	start := p.Offset()
	unsignedTransaction, err := UnmarshalTxData(p, actionRegistry)
	if err != nil {
		return nil, err
	}
	auth, err := authRegistry.Unmarshal(p)
	if err != nil {
		return nil, fmt.Errorf("%w: could not unmarshal auth", err)
	}
	authType := auth.GetTypeID()

	if actorType := auth.Actor()[0]; actorType != authType {
		return nil, fmt.Errorf("%w: actorType (%d) did not match authType (%d)", ErrInvalidActor, actorType, authType)
	}
	if sponsorType := auth.Sponsor()[0]; sponsorType != authType {
		return nil, fmt.Errorf("%w: sponsorType (%d) did not match authType (%d)", ErrInvalidSponsor, sponsorType, authType)
	}

	var tx Transaction
	tx.TransactionData = *unsignedTransaction
	tx.Auth = auth
	if err := p.Err(); err != nil {
		return nil, p.Err()
	}
	codecBytes := p.Bytes()
	tx.bytes = codecBytes[start:p.Offset()] // ensure errors handled before grabbing memory
	tx.size = len(tx.bytes)
	tx.id = utils.ToID(tx.bytes)
	return &tx, nil
}

func MarshalTxs(txs []*Transaction) ([]byte, error) {
	if len(txs) == 0 {
		return nil, ErrNoTxs
	}
	size := consts.IntLen + codec.CummSize(txs)
	p := codec.NewWriter(size, consts.NetworkSizeLimit)
	p.PackInt(uint32(len(txs)))
	for _, tx := range txs {
		if err := tx.Marshal(p); err != nil {
			return nil, err
		}
	}
	return p.Bytes(), p.Err()
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
	for _, action := range actions {
		actionSize, err := GetSize(action)
		if err != nil {
			return fees.Dimensions{}, err
		}

		actor := authFactory.Address()
		stateKeys := action.StateKeys(actor)
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

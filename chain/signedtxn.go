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
	_ emap.Item    = (*SignedTransaction)(nil)
	_ mempool.Item = (*SignedTransaction)(nil)
)

type SignedTransaction struct {
	Transaction

	Auth Auth `json:"auth"`

	bytes     []byte
	size      int
	id        ids.ID
	stateKeys state.Keys
}

func (t *SignedTransaction) Bytes() []byte { return t.bytes }

func (t *SignedTransaction) Size() int { return t.size }

func (t *SignedTransaction) ID() ids.ID { return t.id }

func (t *SignedTransaction) StateKeys(sm StateManager) (state.Keys, error) {
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
	for k, v := range sm.SponsorStateKeys(t.Auth.Sponsor()) {
		if !stateKeys.Add(k, v) {
			return nil, ErrInvalidKeyValue
		}
	}

	// Cache keys if called again
	t.stateKeys = stateKeys
	return stateKeys, nil
}

// Units is charged whether or not a transaction is successful.
func (t *SignedTransaction) Units(sm StateManager, r Rules) (fees.Dimensions, error) {
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
	stateKeys, err := t.StateKeys(sm)
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

func (t *SignedTransaction) PreExecute(
	ctx context.Context,
	feeManager *internalfees.Manager,
	s StateManager,
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
	units, err := t.Units(s, r)
	if err != nil {
		return err
	}
	fee, err := feeManager.Fee(units)
	if err != nil {
		return err
	}
	return s.CanDeduct(ctx, t.Auth.Sponsor(), im, fee)
}

// Execute after knowing a transaction can pay a fee. Attempt
// to charge the fee in as many cases as possible.
//
// Invariant: [PreExecute] is called just before [Execute]
func (t *SignedTransaction) Execute(
	ctx context.Context,
	feeManager *internalfees.Manager,
	s StateManager,
	r Rules,
	ts *tstate.TStateView,
	timestamp int64,
) (*Result, error) {
	// Always charge fee first
	units, err := t.Units(s, r)
	if err != nil {
		// Should never happen
		return nil, err
	}
	fee, err := feeManager.Fee(units)
	if err != nil {
		// Should never happen
		return nil, err
	}
	if err := s.Deduct(ctx, t.Auth.Sponsor(), ts, fee); err != nil {
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
func (t *SignedTransaction) Sponsor() codec.Address { return t.Auth.Sponsor() }

func (t *SignedTransaction) Marshal(p *codec.Packer) error {
	if len(t.bytes) > 0 {
		p.PackFixedBytes(t.bytes)
		return p.Err()
	}
	return t.marshal(p)
}

func (t *SignedTransaction) marshal(p *codec.Packer) error {
	if err := t.Transaction.marshal(p); err != nil {
		return err
	}

	authID := t.Auth.GetTypeID()
	p.PackByte(authID)
	t.Auth.Marshal(p)

	return p.Err()
}

// Verify that the transaction was signed correctly.
func (t *SignedTransaction) Verify(ctx context.Context) error {
	msg, err := t.Transaction.Bytes()
	if err != nil {
		// Should never occur because populated during unmarshal
		return err
	}
	return t.Auth.Verify(ctx, msg)
}

func MarshalSignedTxs(txs []*SignedTransaction) ([]byte, error) {
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

func UnmarshalSignedTxs(
	raw []byte,
	initialCapacity int,
	actionRegistry ActionRegistry,
	authRegistry AuthRegistry,
) (map[uint8]int, []*SignedTransaction, error) {
	p := codec.NewReader(raw, consts.NetworkSizeLimit)
	txCount := p.UnpackInt(true)
	authCounts := map[uint8]int{}
	txs := make([]*SignedTransaction, 0, initialCapacity) // DoS to set size to txCount
	for i := uint32(0); i < txCount; i++ {
		tx, err := UnmarshalSignedTx(p, actionRegistry, authRegistry)
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

func UnmarshalSignedTx(
	p *codec.Packer,
	actionRegistry *codec.TypeParser[Action],
	authRegistry *codec.TypeParser[Auth],
) (*SignedTransaction, error) {
	start := p.Offset()
	unsignedTransaction, err := UnmarshalTx(p, actionRegistry)
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

	var tx SignedTransaction
	tx.Transaction = *unsignedTransaction
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

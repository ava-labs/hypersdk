// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/math"
	"github.com/ava-labs/hypersdk/mempool"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/tstate"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	_ emap.Item    = (*Transaction)(nil)
	_ mempool.Item = (*Transaction)(nil)
)

type Transaction struct {
	Base *Base `json:"base"`

	Actions []Action `json:"actions"`
	Auth    Auth     `json:"auth"`

	digest    []byte
	bytes     []byte
	size      int
	id        ids.ID
	stateKeys state.Keys
}

func NewTx(base *Base, actions []Action) *Transaction {
	return &Transaction{
		Base:    base,
		Actions: actions,
	}
}

func (t *Transaction) Digest() ([]byte, error) {
	if len(t.digest) > 0 {
		return t.digest, nil
	}
	size := t.Base.Size()
	for _, action := range t.Actions {
		size += consts.ByteLen + action.Size()
	}
	p := codec.NewWriter(size, consts.NetworkSizeLimit)
	t.Base.Marshal(p)
	p.PackInt(len(t.Actions))
	for _, action := range t.Actions {
		p.PackByte(action.GetTypeID())
		action.Marshal(p)
	}
	return p.Bytes(), p.Err()
}

func (t *Transaction) Sign(
	factory AuthFactory,
	actionRegistry ActionRegistry,
	authRegistry AuthRegistry,
) (*Transaction, error) {
	msg, err := t.Digest()
	if err != nil {
		return nil, err
	}
	auth, err := factory.Sign(msg)
	if err != nil {
		return nil, err
	}
	t.Auth = auth

	// Ensure transaction is fully initialized and correct by reloading it from
	// bytes
	size := len(msg) + consts.ByteLen + t.Auth.Size()
	p := codec.NewWriter(size, consts.NetworkSizeLimit)
	if err := t.Marshal(p); err != nil {
		return nil, err
	}
	if err := p.Err(); err != nil {
		return nil, err
	}
	p = codec.NewReader(p.Bytes(), consts.MaxInt)
	return UnmarshalTx(p, actionRegistry, authRegistry)
}

func (t *Transaction) Bytes() []byte { return t.bytes }

func (t *Transaction) Size() int { return t.size }

func (t *Transaction) ID() ids.ID { return t.id }

func (t *Transaction) Expiry() int64 { return t.Base.Timestamp }

func (t *Transaction) MaxFee() uint64 { return t.Base.MaxFee }

func (t *Transaction) StateKeys(sm StateManager) (state.Keys, error) {
	if t.stateKeys != nil {
		return t.stateKeys, nil
	}
	stateKeys := make(state.Keys)

	// Verify the formatting of state keys passed by the controller
	for i, action := range t.Actions {
		actionKeys := action.StateKeys(t.Auth.Actor(), codec.CreateLID(uint8(i), t.id))
		for k, v := range actionKeys {
			if !keys.Valid(k) {
				return nil, ErrInvalidKeyValue
			}
			// [Add] will take the union of key permissions
			stateKeys.Add(k, v)
		}
	}
	sponsorKeys := sm.SponsorStateKeys(t.Auth.Sponsor())
	for k, v := range sponsorKeys {
		if !keys.Valid(k) {
			return nil, ErrInvalidKeyValue
		}
		stateKeys.Add(k, v)
	}

	// Cache keys if called again
	t.stateKeys = stateKeys
	return stateKeys, nil
}

// Sponsor is the [codec.Address] that pays fees for this transaction.
func (t *Transaction) Sponsor() codec.Address { return t.Auth.Sponsor() }

// Units is charged whether or not a transaction is successful because state
// lookup is not free.
func (t *Transaction) MaxUnits(sm StateManager, r Rules) (fees.Dimensions, error) {
	// Cacluate max compute costs
	maxComputeUnitsOp := math.NewUint64Operator(r.GetBaseComputeUnits())
	for _, action := range t.Actions {
		maxComputeUnitsOp.Add(action.MaxComputeUnits(r))
	}
	maxComputeUnitsOp.Add(t.Auth.ComputeUnits(r))
	maxComputeUnits, err := maxComputeUnitsOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}

	// Calculate the max storage cost we could incur by processing all
	// state keys.
	//
	// TODO: make this a tighter bound (allow for granular storage controls)
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

// EstimateMaxUnits provides a pessimistic estimate of the cost to execute a transaction. This is
// typically used during transaction construction.
func EstimateMaxUnits(r Rules, actions []Action, authFactory AuthFactory) (fees.Dimensions, error) {
	authBandwidth, authCompute := authFactory.MaxUnits()
	bandwidth := BaseSize + consts.ByteLen + consts.ByteLen + authBandwidth
	computeUnitsOp := math.NewUint64Operator(r.GetBaseComputeUnits())
	computeUnitsOp.Add(authCompute)

	stateKeysMaxChunks := []uint16{}
	sponsorStateKeyMaxChunks := r.GetSponsorStateKeysMaxChunks()
	stateKeysMaxChunks = append(stateKeysMaxChunks, sponsorStateKeyMaxChunks...)
	for _, action := range actions {
		bandwidth += uint64(action.Size())
		actionStateKeysMaxChunks := action.StateKeysMaxChunks()
		stateKeysMaxChunks = append(stateKeysMaxChunks, actionStateKeysMaxChunks...)

		computeUnitsOp.Add(action.MaxComputeUnits(r))
	}

	// Estimate compute costs
	computeUnits, err := computeUnitsOp.Value()
	if err != nil {
		return fees.Dimensions{}, err
	}

	// Estimate storage costs
	//
	// TODO: unify this with [MaxUnits] handling
	readsOp := math.NewUint64Operator(0)
	allocatesOp := math.NewUint64Operator(0)
	writesOp := math.NewUint64Operator(0)
	for maxChunks := range stateKeysMaxChunks {
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
	return fees.Dimensions{bandwidth, computeUnits, reads, allocates, writes}, nil
}

func (t *Transaction) PreExecute(
	ctx context.Context,
	feeManager *fees.Manager,
	s StateManager,
	r Rules,
	im state.Immutable,
	timestamp int64,
) error {
	if err := t.Base.Execute(r.ChainID(), r, timestamp); err != nil {
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
	maxUnits, err := t.MaxUnits(s, r)
	if err != nil {
		return err
	}
	maxFee, err := feeManager.MaxFee(maxUnits)
	if err != nil {
		return err
	}
	return s.CanDeduct(ctx, t.Auth.Sponsor(), im, maxFee)
}

// Execute after knowing a transaction can pay a fee. Attempt
// to charge the fee in as many cases as possible.
//
// ColdRead/WarmRead size is based on the amount read BEFORE
// the block begins.
//
// Invariant: [PreExecute] is called just before [Execute]
func (t *Transaction) Execute(
	ctx context.Context,
	feeManager *fees.Manager,
	reads map[string]uint16,
	s StateManager,
	r Rules,
	ts *tstate.TStateView,
	timestamp int64,
) (*Result, error) {
	// Always charge fee first (in case [Action] moves funds)
	maxUnits, err := t.MaxUnits(s, r)
	if err != nil {
		// Should never happen
		return nil, err
	}
	maxFee, err := feeManager.MaxFee(maxUnits)
	if err != nil {
		// Should never happen
		return nil, err
	}
	if err := s.Deduct(ctx, t.Auth.Sponsor(), ts, maxFee); err != nil {
		// This should never fail for low balance (as we check [CanDeductFee]
		// immediately before).
		return nil, err
	}

	// We create a temp state checkpoint to ensure we don't commit failed actions to state.
	actionStart := ts.OpIndex()
	resultOutputs := [][][]byte{}
	handleRevert := func(rerr error) (*Result, error) {
		// Be warned that the variables captured in this function
		// are set when this function is defined. If any of them are
		// modified later, they will not be used here.
		//
		// Note: Revert will not return an error per action.
		ts.Rollback(ctx, actionStart)

		// include error in the last action
		resultOutputs[len(resultOutputs)-1] = append(resultOutputs[len(resultOutputs)-1], utils.ErrBytes(rerr))
		return &Result{false, resultOutputs, maxUnits, maxFee}, nil
	}

	var (
		computeUnitsOp = math.NewUint64Operator(r.GetBaseComputeUnits())
		txSuccess      = true
	)
	for i, action := range t.Actions {
		// skip all following actions if one is unsuccessful
		if !txSuccess {
			break
		}
		actionID := codec.CreateLID(uint8(i), t.id)
		success, actionCUs, outputs := action.Execute(ctx, r, ts, timestamp, t.Auth.Actor(), actionID)
		if len(outputs) == 0 && outputs != nil {
			// Enforce object standardization (this is a VM bug and we should fail
			// fast)
			return handleRevert(ErrInvalidObject)
		}
		resultOutputs = append(resultOutputs, outputs)
		if len(outputs) > int(r.GetMaxOutputsPerAction()) {
			return handleRevert(ErrTooManyOutputs)
		}

		if !success {
			txSuccess = false
			ts.Rollback(ctx, actionStart)
		}

		// Calculate units used
		computeUnitsOp.Add(actionCUs)
	}
	computeUnitsOp.Add(t.Auth.ComputeUnits(r))
	computeUnits, err := computeUnitsOp.Value()
	if err != nil {
		return handleRevert(err)
	}

	// Because the key database is abstracted from [Auth]/[Actions], we can compute
	// all storage use in the background. KeyOperations is unique to a view.
	allocates, writes := ts.KeyOperations()

	// Because we compute the fee before [Auth.Refund] is called, we need
	// to pessimistically precompute the storage it will change.
	for key := range s.SponsorStateKeys(t.Auth.Sponsor()) {
		// maxChunks will be greater than the chunks read in any of these keys,
		// so we don't need to check for pre-existing values.
		maxChunks, ok := keys.MaxChunks([]byte(key))
		if !ok {
			return handleRevert(ErrInvalidKeyValue)
		}
		writes[key] = maxChunks
	}

	// We only charge for the chunks read from disk instead of charging for the max chunks
	// specified by the key.
	readsOp := math.NewUint64Operator(0)
	for _, chunksRead := range reads {
		readsOp.Add(r.GetStorageKeyReadUnits())
		readsOp.MulAdd(uint64(chunksRead), r.GetStorageValueReadUnits())
	}
	readUnits, err := readsOp.Value()
	if err != nil {
		return handleRevert(err)
	}
	allocatesOp := math.NewUint64Operator(0)
	for _, chunksStored := range allocates {
		allocatesOp.Add(r.GetStorageKeyAllocateUnits())
		allocatesOp.MulAdd(uint64(chunksStored), r.GetStorageValueAllocateUnits())
	}
	allocateUnits, err := allocatesOp.Value()
	if err != nil {
		return handleRevert(err)
	}
	writesOp := math.NewUint64Operator(0)
	for _, chunksModified := range writes {
		writesOp.Add(r.GetStorageKeyWriteUnits())
		writesOp.MulAdd(uint64(chunksModified), r.GetStorageValueWriteUnits())
	}
	writeUnits, err := writesOp.Value()
	if err != nil {
		return handleRevert(err)
	}
	used := fees.Dimensions{uint64(t.Size()), computeUnits, readUnits, allocateUnits, writeUnits}
	if err != nil {
		return handleRevert(err)
	}

	// Check to see if the units consumed are greater than the max units
	//
	// This should never be the case but erroring here is better than
	// underflowing the refund.
	if !maxUnits.Greater(used) {
		return nil, fmt.Errorf("%w: max=%+v consumed=%+v", ErrInvalidUnitsConsumed, maxUnits, used)
	}

	// Return any funds from unused units
	//
	// To avoid storage abuse of [Auth.Refund], we precharge for possible usage.
	feeRequired, err := feeManager.MaxFee(used)
	if err != nil {
		return handleRevert(err)
	}
	refund := maxFee - feeRequired
	if refund > 0 {
		ts.DisableAllocation()
		err = s.Refund(ctx, t.Auth.Sponsor(), ts, refund)
		ts.EnableAllocation()
		if err != nil {
			return handleRevert(err)
		}
	}
	return &Result{
		Success: txSuccess,
		Outputs: resultOutputs,

		Consumed: used,
		Fee:      feeRequired,
	}, nil
}

func (t *Transaction) Marshal(p *codec.Packer) error {
	if len(t.bytes) > 0 {
		p.PackFixedBytes(t.bytes)
		return p.Err()
	}

	return t.marshalActions(p)
}

func (t *Transaction) marshalActions(p *codec.Packer) error {
	t.Base.Marshal(p)
	p.PackInt(len(t.Actions))
	for _, action := range t.Actions {
		actionID := action.GetTypeID()
		p.PackByte(actionID)
		action.Marshal(p)
	}
	authID := t.Auth.GetTypeID()
	p.PackByte(authID)
	t.Auth.Marshal(p)
	return p.Err()
}

func MarshalTxs(txs []*Transaction) ([]byte, error) {
	if len(txs) == 0 {
		return nil, ErrNoTxs
	}
	size := consts.IntLen + codec.CummSize(txs)
	p := codec.NewWriter(size, consts.NetworkSizeLimit)
	p.PackInt(len(txs))
	for _, tx := range txs {
		if err := tx.Marshal(p); err != nil {
			return nil, err
		}
	}
	return p.Bytes(), p.Err()
}

func UnmarshalTxs(
	raw []byte,
	initialCapacity int,
	actionRegistry ActionRegistry,
	authRegistry AuthRegistry,
) (map[uint8]int, []*Transaction, error) {
	p := codec.NewReader(raw, consts.NetworkSizeLimit)
	txCount := p.UnpackInt(true)
	authCounts := map[uint8]int{}
	txs := make([]*Transaction, 0, initialCapacity) // DoS to set size to txCount
	for i := 0; i < txCount; i++ {
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
	actionRegistry *codec.TypeParser[Action, bool],
	authRegistry *codec.TypeParser[Auth, bool],
) (*Transaction, error) {
	start := p.Offset()
	base, err := UnmarshalBase(p)
	if err != nil {
		return nil, fmt.Errorf("%w: could not unmarshal base", err)
	}
	actions, err := unmarshalActions(p, actionRegistry)
	if err != nil {
		return nil, fmt.Errorf("%w: could not unmarshal actions", err)
	}
	digest := p.Offset()
	authType := p.UnpackByte()
	unmarshalAuth, ok := authRegistry.LookupIndex(authType)
	if !ok {
		return nil, fmt.Errorf("%w: %d is unknown auth type", ErrInvalidObject, authType)
	}
	auth, err := unmarshalAuth(p)
	if err != nil {
		return nil, fmt.Errorf("%w: could not unmarshal auth", err)
	}
	if actorType := auth.Actor()[0]; actorType != authType {
		return nil, fmt.Errorf("%w: actorType (%d) did not match authType (%d)", ErrInvalidActor, actorType, authType)
	}
	if sponsorType := auth.Sponsor()[0]; sponsorType != authType {
		return nil, fmt.Errorf("%w: sponsorType (%d) did not match authType (%d)", ErrInvalidSponsor, sponsorType, authType)
	}

	var tx Transaction
	tx.Base = base
	tx.Actions = actions
	tx.Auth = auth
	if err := p.Err(); err != nil {
		return nil, p.Err()
	}
	codecBytes := p.Bytes()
	tx.digest = codecBytes[start:digest]
	tx.bytes = codecBytes[start:p.Offset()] // ensure errors handled before grabbing memory
	tx.size = len(tx.bytes)
	tx.id = utils.ToID(tx.bytes)
	return &tx, nil
}

func unmarshalActions(
	p *codec.Packer,
	actionRegistry *codec.TypeParser[Action, bool],
) ([]Action, error) {
	actionCount := p.UnpackInt(true)
	actions := make([]Action, 0)

	for i := 0; i < actionCount; i++ {
		actionType := p.UnpackByte()
		unmarshalAction, ok := actionRegistry.LookupIndex(actionType)
		if !ok {
			return nil, fmt.Errorf("%w: %d is unknown action type", ErrInvalidObject, actionType)
		}

		action, err := unmarshalAction(p)
		if err != nil {
			return nil, fmt.Errorf("%w: could not unmarshal action", err)
		}
		actions = append(actions, action)
	}
	return actions, nil
}

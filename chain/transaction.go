// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/emap"
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
	Base        *Base         `json:"base"`
	WarpMessage *warp.Message `json:"warpMessage"`
	Action      Action        `json:"action"`
	Auth        Auth          `json:"auth"`

	digest         []byte
	bytes          []byte
	size           int
	id             ids.ID
	numWarpSigners int
	// warpID is just the hash of the *warp.Message.Payload. We assumed that
	// all warp messages from a single source have some unique field that
	// prevents duplicates (like txID). We will not allow 2 instances of the same
	// warpID from the same sourceChainID to be accepted.
	warpID    ids.ID
	stateKeys set.Set[string]
}

type WarpResult struct {
	Message   *warp.Message
	VerifyErr error
}

func NewTx(base *Base, wm *warp.Message, act Action) *Transaction {
	return &Transaction{
		Base:        base,
		WarpMessage: wm,
		Action:      act,
	}
}

func (t *Transaction) Digest() ([]byte, error) {
	if len(t.digest) > 0 {
		return t.digest, nil
	}
	actionID := t.Action.GetTypeID()
	var warpBytes []byte
	if t.WarpMessage != nil {
		warpBytes = t.WarpMessage.Bytes()
	}
	size := t.Base.Size() +
		codec.BytesLen(warpBytes) +
		consts.ByteLen + t.Action.Size()
	p := codec.NewWriter(size, consts.NetworkSizeLimit)
	t.Base.Marshal(p)
	p.PackBytes(warpBytes)
	p.PackByte(actionID)
	t.Action.Marshal(p)
	return p.Bytes(), p.Err()
}

func (t *Transaction) Sign(
	factory AuthFactory,
	actionRegistry ActionRegistry,
	authRegistry AuthRegistry,
) (*Transaction, error) {
	// Generate auth
	msg, err := t.Digest()
	if err != nil {
		return nil, err
	}
	auth, err := factory.Sign(msg, t.Action)
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

func (t *Transaction) AuthAsyncVerify() func() error {
	return func() error {
		// It is up to [t.Auth] to limit the computational
		// complexity of [t.Auth.AsyncVerify] and [t.Auth.Verify] to prevent
		// a DoS (invalid Auth will not charge [t.Auth.Sponsor()].
		return t.Auth.AsyncVerify(t.digest)
	}
}

func (t *Transaction) Bytes() []byte { return t.bytes }

func (t *Transaction) Size() int { return t.size }

func (t *Transaction) ID() ids.ID { return t.id }

func (t *Transaction) Expiry() int64 { return t.Base.Timestamp }

func (t *Transaction) MaxFee() uint64 { return t.Base.MaxFee }

func (t *Transaction) StateKeys(stateMapping StateManager) (set.Set[string], error) {
	if t.stateKeys != nil {
		return t.stateKeys, nil
	}

	// Verify the formatting of state keys passed by the controller
	actionKeys := t.Action.StateKeys(t.Auth, t.ID())
	authKeys := t.Auth.StateKeys()
	stateKeys := set.NewSet[string](len(actionKeys) + len(authKeys))
	for _, arr := range [][]string{actionKeys, authKeys} {
		for _, k := range arr {
			if !keys.Valid(k) {
				return nil, ErrInvalidKeyValue
			}
			stateKeys.Add(k)
		}
	}

	// Add keys used to manage warp operations
	if t.WarpMessage != nil {
		p := stateMapping.IncomingWarpKeyPrefix(t.WarpMessage.SourceChainID, t.warpID)
		k := keys.EncodeChunks(p, MaxIncomingWarpChunks)
		stateKeys.Add(string(k))
	}
	if t.Action.OutputsWarpMessage() {
		p := stateMapping.OutgoingWarpKeyPrefix(t.id)
		k := keys.EncodeChunks(p, MaxOutgoingWarpChunks)
		stateKeys.Add(string(k))
	}

	// Cache keys if called again
	t.stateKeys = stateKeys
	return stateKeys, nil
}

// Units is charged whether or not a transaction is successful because state
// lookup is not free.
func (t *Transaction) MaxUnits(sm StateManager, r Rules) (Dimensions, error) {
	// Cacluate max compute costs
	maxComputeUnitsOp := math.NewUint64Operator(r.GetBaseComputeUnits())
	maxComputeUnitsOp.Add(t.Action.MaxComputeUnits(r))
	maxComputeUnitsOp.Add(t.Auth.MaxComputeUnits(r))
	if t.WarpMessage != nil {
		maxComputeUnitsOp.Add(r.GetBaseWarpComputeUnits())
		maxComputeUnitsOp.MulAdd(uint64(t.numWarpSigners), r.GetWarpComputeUnitsPerSigner())
	}
	if t.Action.OutputsWarpMessage() {
		// Chunks later accounted for by call to [StateKeys]
		maxComputeUnitsOp.Add(r.GetOutgoingWarpComputeUnits())
	}
	maxComputeUnits, err := maxComputeUnitsOp.Value()
	if err != nil {
		return Dimensions{}, err
	}

	// Calculate the max storage cost we could incur by processing all
	// state keys.
	//
	// TODO: make this a tighter bound (allow for granular storage controls)
	stateKeys, err := t.StateKeys(sm)
	if err != nil {
		return Dimensions{}, err
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
			return Dimensions{}, ErrInvalidKeyValue
		}
		readsOp.MulAdd(uint64(maxChunks), r.GetStorageValueReadUnits())
		allocatesOp.MulAdd(uint64(maxChunks), r.GetStorageValueAllocateUnits())
		writesOp.MulAdd(uint64(maxChunks), r.GetStorageValueWriteUnits())
	}
	reads, err := readsOp.Value()
	if err != nil {
		return Dimensions{}, err
	}
	allocates, err := allocatesOp.Value()
	if err != nil {
		return Dimensions{}, err
	}
	writes, err := writesOp.Value()
	if err != nil {
		return Dimensions{}, err
	}
	return Dimensions{uint64(t.Size()), maxComputeUnits, reads, allocates, writes}, nil
}

// EstimateMaxUnits provides a pessimistic estimate of the cost to execute a transaction. This is
// typically used during transaction construction.
func EstimateMaxUnits(r Rules, action Action, authFactory AuthFactory, warpMessage *warp.Message) (Dimensions, error) {
	authBandwidth, authCompute, authStateKeysMaxChunks := authFactory.MaxUnits()
	bandwidth := BaseSize + consts.ByteLen + uint64(action.Size()) + consts.ByteLen + authBandwidth
	actionStateKeysMaxChunks := action.StateKeysMaxChunks()
	stateKeysMaxChunks := make([]uint16, 0, len(authStateKeysMaxChunks)+len(actionStateKeysMaxChunks))
	stateKeysMaxChunks = append(stateKeysMaxChunks, authStateKeysMaxChunks...)
	stateKeysMaxChunks = append(stateKeysMaxChunks, actionStateKeysMaxChunks...)

	// Estimate compute costs
	computeUnitsOp := math.NewUint64Operator(r.GetBaseComputeUnits())
	computeUnitsOp.Add(authCompute)
	computeUnitsOp.Add(action.MaxComputeUnits(r))
	if warpMessage != nil {
		bandwidth += uint64(codec.BytesLen(warpMessage.Bytes()))
		stateKeysMaxChunks = append(stateKeysMaxChunks, MaxIncomingWarpChunks)
		computeUnitsOp.Add(r.GetBaseWarpComputeUnits())
		numSigners, err := warpMessage.Signature.NumSigners()
		if err != nil {
			return Dimensions{}, err
		}
		computeUnitsOp.MulAdd(uint64(numSigners), r.GetWarpComputeUnitsPerSigner())
	}
	if action.OutputsWarpMessage() {
		stateKeysMaxChunks = append(stateKeysMaxChunks, MaxOutgoingWarpChunks)
		computeUnitsOp.Add(r.GetOutgoingWarpComputeUnits())
	}
	computeUnits, err := computeUnitsOp.Value()
	if err != nil {
		return Dimensions{}, err
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
		return Dimensions{}, err
	}
	allocates, err := allocatesOp.Value()
	if err != nil {
		return Dimensions{}, err
	}
	writes, err := writesOp.Value()
	if err != nil {
		return Dimensions{}, err
	}
	return Dimensions{bandwidth, computeUnits, reads, allocates, writes}, nil
}

func (t *Transaction) PreExecute(
	ctx context.Context,
	feeManager *FeeManager,
	s StateManager,
	r Rules,
	im state.Immutable,
	timestamp int64,
) (uint64, error) {
	if err := t.Base.Execute(r.ChainID(), r, timestamp); err != nil {
		return 0, err
	}
	start, end := t.Action.ValidRange(r)
	if start >= 0 && timestamp < start {
		return 0, ErrActionNotActivated
	}
	if end >= 0 && timestamp > end {
		return 0, ErrActionNotActivated
	}
	start, end = t.Auth.ValidRange(r)
	if start >= 0 && timestamp < start {
		return 0, ErrAuthNotActivated
	}
	if end >= 0 && timestamp > end {
		return 0, ErrAuthNotActivated
	}
	// It is up to [t.Auth] to limit the computational
	// complexity of [t.Auth.AsyncVerify] and [t.Auth.Verify] to prevent
	// a DoS (invalid Auth will not charge [t.Auth.Sponsor()].
	authCUs, err := t.Auth.Verify(ctx, r, im, t.Action)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrAuthFailed, err) //nolint:errorlint
	}
	maxUnits, err := t.MaxUnits(s, r)
	if err != nil {
		return 0, err
	}
	maxFee, err := feeManager.MaxFee(maxUnits)
	if err != nil {
		return 0, err
	}
	if err := t.Auth.CanDeduct(ctx, im, maxFee); err != nil {
		return 0, err
	}
	return authCUs, nil
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
	feeManager *FeeManager,
	authCUs uint64,
	reads map[string]uint16,
	s StateManager,
	r Rules,
	ts *tstate.TStateView,
	timestamp int64,
	warpVerified bool,
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
	if err := t.Auth.Deduct(ctx, ts, maxFee); err != nil {
		// This should never fail for low balance (as we check [CanDeductFee]
		// immediately before).
		return nil, err
	}

	// Check warp message is not duplicate
	if t.WarpMessage != nil {
		p := s.IncomingWarpKeyPrefix(t.WarpMessage.SourceChainID, t.warpID)
		k := keys.EncodeChunks(p, MaxIncomingWarpChunks)
		_, err := ts.GetValue(ctx, k)
		switch {
		case err == nil:
			// Override all errors because warp message is a duplicate
			warpVerified = false
		case errors.Is(err, database.ErrNotFound):
			// This means there are no conflicts
		case err != nil:
			// An error here can indicate there is an issue with the database or that
			// the key was not properly specified.
			return &Result{false, utils.ErrBytes(err), maxUnits, maxFee, nil}, nil
		}
	}

	// We create a temp state checkpoint to ensure we don't commit failed actions to state.
	actionStart := ts.OpIndex()
	handleRevert := func(rerr error) (*Result, error) {
		// Be warned that the variables captured in this function
		// are set when this function is defined. If any of them are
		// modified later, they will not be used here.
		ts.Rollback(ctx, actionStart)
		return &Result{false, utils.ErrBytes(rerr), maxUnits, maxFee, nil}, nil
	}
	success, actionCUs, output, warpMessage, err := t.Action.Execute(ctx, r, ts, timestamp, t.Auth, t.id, warpVerified)
	if err != nil {
		return handleRevert(err)
	}
	if len(output) == 0 && output != nil {
		// Enforce object standardization (this is a VM bug and we should fail
		// fast)
		return handleRevert(ErrInvalidObject)
	}
	outputsWarp := t.Action.OutputsWarpMessage()
	if !success {
		ts.Rollback(ctx, actionStart)
		warpMessage = nil // warp messages can only be emitted on success
	} else {
		// Ensure constraints hold if successful
		if (warpMessage == nil && outputsWarp) || (warpMessage != nil && !outputsWarp) {
			return handleRevert(ErrInvalidObject)
		}

		// Store incoming warp messages in state by their ID to prevent replays
		if t.WarpMessage != nil {
			p := s.IncomingWarpKeyPrefix(t.WarpMessage.SourceChainID, t.warpID)
			k := keys.EncodeChunks(p, MaxIncomingWarpChunks)
			if err := ts.Insert(ctx, k, nil); err != nil {
				return handleRevert(err)
			}
		}

		// Store newly created warp messages in state by their txID to ensure we can
		// always sign for a message
		if warpMessage != nil {
			// Enforce we are the source of our own messages
			warpMessage.NetworkID = r.NetworkID()
			warpMessage.SourceChainID = r.ChainID()
			// Initialize message (compute bytes) now that everything is populated
			if err := warpMessage.Initialize(); err != nil {
				return handleRevert(err)
			}
			// We use txID here because did not know the warpID before execution (and
			// we pre-reserve this key for the processor).
			p := s.OutgoingWarpKeyPrefix(t.id)
			k := keys.EncodeChunks(p, MaxOutgoingWarpChunks)
			if err := ts.Insert(ctx, k, warpMessage.Bytes()); err != nil {
				return handleRevert(err)
			}
		}
	}

	// Calculate units used
	computeUnitsOp := math.NewUint64Operator(r.GetBaseComputeUnits())
	computeUnitsOp.Add(authCUs)
	computeUnitsOp.Add(actionCUs)
	if t.WarpMessage != nil {
		computeUnitsOp.Add(r.GetBaseWarpComputeUnits())
		computeUnitsOp.MulAdd(uint64(t.numWarpSigners), r.GetWarpComputeUnitsPerSigner())
	}
	if success && outputsWarp {
		computeUnitsOp.Add(r.GetOutgoingWarpComputeUnits())
	}
	computeUnits, err := computeUnitsOp.Value()
	if err != nil {
		return handleRevert(err)
	}

	// Because the key database is abstracted from [Auth]/[Actions], we can compute
	// all storage use in the background. KeyOperations is unique to a view.
	allocates, writes := ts.KeyOperations()

	// Because we compute the fee before [Auth.Refund] is called, we need
	// to pessimistically precompute the storage it will change.
	for _, key := range t.Auth.StateKeys() {
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
	used := Dimensions{uint64(t.Size()), computeUnits, readUnits, allocateUnits, writeUnits}

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
		defer ts.EnableAllocation()
		if err := t.Auth.Refund(ctx, ts, refund); err != nil {
			return handleRevert(err)
		}
	}
	return &Result{
		Success: success,
		Output:  output,

		Consumed: used,
		Fee:      feeRequired,

		WarpMessage: warpMessage,
	}, nil
}

func (t *Transaction) Sponsor() codec.Address {
	return t.Auth.Sponsor()
}

func (t *Transaction) Marshal(p *codec.Packer) error {
	if len(t.bytes) > 0 {
		p.PackFixedBytes(t.bytes)
		return p.Err()
	}

	actionID := t.Action.GetTypeID()
	authID := t.Auth.GetTypeID()
	t.Base.Marshal(p)
	var warpBytes []byte
	if t.WarpMessage != nil {
		warpBytes = t.WarpMessage.Bytes()
		if len(warpBytes) == 0 {
			return ErrWarpMessageNotInitialized
		}
	}
	p.PackBytes(warpBytes)
	p.PackByte(actionID)
	t.Action.Marshal(p)
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
	actionRegistry *codec.TypeParser[Action, *warp.Message, bool],
	authRegistry *codec.TypeParser[Auth, *warp.Message, bool],
) (*Transaction, error) {
	start := p.Offset()
	base, err := UnmarshalBase(p)
	if err != nil {
		return nil, fmt.Errorf("%w: could not unmarshal base", err)
	}
	var warpBytes []byte
	p.UnpackBytes(MaxWarpMessageSize, false, &warpBytes)
	var warpMessage *warp.Message
	var numWarpSigners int
	if len(warpBytes) > 0 {
		msg, err := warp.ParseMessage(warpBytes)
		if err != nil {
			return nil, fmt.Errorf("%w: could not unmarshal warp message", err)
		}
		if len(msg.Payload) == 0 {
			return nil, ErrEmptyWarpPayload
		}
		warpMessage = msg
		numSigners, err := msg.Signature.NumSigners()
		if err != nil {
			return nil, fmt.Errorf("%w: could not calculate number of warp signers", err)
		}
		numWarpSigners = numSigners
	}
	actionType := p.UnpackByte()
	unmarshalAction, actionWarp, ok := actionRegistry.LookupIndex(actionType)
	if !ok {
		return nil, fmt.Errorf("%w: %d is unknown action type", ErrInvalidObject, actionType)
	}
	if actionWarp && warpMessage == nil {
		return nil, fmt.Errorf("%w: action %d", ErrExpectedWarpMessage, actionType)
	}
	action, err := unmarshalAction(p, warpMessage)
	if err != nil {
		return nil, fmt.Errorf("%w: could not unmarshal action", err)
	}
	digest := p.Offset()
	authType := p.UnpackByte()
	unmarshalAuth, authWarp, ok := authRegistry.LookupIndex(authType)
	if !ok {
		return nil, fmt.Errorf("%w: %d is unknown auth type", ErrInvalidObject, authType)
	}
	if authWarp && warpMessage == nil {
		return nil, fmt.Errorf("%w: auth %d", ErrExpectedWarpMessage, authType)
	}
	auth, err := unmarshalAuth(p, warpMessage)
	if err != nil {
		return nil, fmt.Errorf("%w: could not unmarshal auth", err)
	}
	if actorType := auth.Actor()[0]; actorType != authType {
		return nil, fmt.Errorf("%w: actorType (%d) did not match authType (%d)", ErrInvalidActor, actorType, authType)
	}
	if sponsorType := auth.Sponsor()[0]; sponsorType != authType {
		return nil, fmt.Errorf("%w: sponsorType (%d) did not match authType (%d)", ErrInvalidSponsor, sponsorType, authType)
	}
	warpExpected := actionWarp || authWarp
	if !warpExpected && warpMessage != nil {
		return nil, ErrUnexpectedWarpMessage
	}

	var tx Transaction
	tx.Base = base
	tx.Action = action
	tx.WarpMessage = warpMessage
	tx.Auth = auth
	if err := p.Err(); err != nil {
		return nil, p.Err()
	}
	codecBytes := p.Bytes()
	tx.digest = codecBytes[start:digest]
	tx.bytes = codecBytes[start:p.Offset()] // ensure errors handled before grabbing memory
	tx.size = len(tx.bytes)
	tx.id = utils.ToID(tx.bytes)
	if tx.WarpMessage != nil {
		tx.numWarpSigners = numWarpSigners
		tx.warpID = tx.WarpMessage.ID()
	}
	return &tx, nil
}

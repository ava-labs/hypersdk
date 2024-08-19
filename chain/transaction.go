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

	// TODO: turn [Action] into an array (#335)
	Action Action `json:"action"`
	Auth   Auth   `json:"auth"`

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
	stateKeys state.Keys
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

func (t *Transaction) Partition() uint8 { return t.Base.Partition }

func (t *Transaction) StateKeys(sm StateManager) (state.Keys, error) {
	if t.stateKeys != nil {
		return t.stateKeys, nil
	}

	// Verify the formatting of state keys passed by the controller
	actionKeys := t.Action.StateKeys(t.Auth.Actor(), t.ID())
	sponsorKeys := sm.SponsorStateKeys(t.Auth.Sponsor())
	stateKeys := make(state.Keys)
	for _, m := range []state.Keys{actionKeys, sponsorKeys} {
		for k, v := range m {
			if !keys.Valid(k) {
				fmt.Printf("action key not valid: %s\n", t.ID().String())
				return nil, ErrInvalidKeyValue
			}
			stateKeys.Add(k, v)
		}
	}

	// Add keys used to manage warp operations
	if t.WarpMessage != nil {
		p := sm.IncomingWarpKeyPrefix(t.WarpMessage.SourceChainID, t.warpID)
		k := keys.EncodeChunks(p, MaxIncomingWarpChunks)
		stateKeys.Add(k, state.Read|state.Write)
	}
	if t.Action.OutputsWarpMessage() {
		p := sm.OutgoingWarpKeyPrefix(t.id)
		k := keys.EncodeChunks(p, MaxOutgoingWarpChunks)
		stateKeys.Add(k, state.Write)
	}

	// Cache keys if called again
	t.stateKeys = stateKeys
	return stateKeys, nil
}

// Sponsor is the [codec.Address] that pays fees for this transaction.
func (t *Transaction) Sponsor() codec.Address { return t.Auth.Sponsor() }

// Units are charged whether or not the transaction is successful and whether or not
// keys specified are accessed. This is done to ensure chunk producers accrue expected
// fees for filling chunks.
func (t *Transaction) Units(sm StateManager, r Rules) (Dimensions, error) {
	// Cacluate compute cost
	computeUnitsOp := math.NewUint64Operator(r.GetBaseComputeUnits())
	computeUnitsOp.Add(t.Action.ComputeUnits(r))
	computeUnitsOp.Add(t.Auth.ComputeUnits(r))
	if t.WarpMessage != nil {
		computeUnitsOp.Add(r.GetBaseWarpComputeUnits())
		computeUnitsOp.MulAdd(uint64(t.numWarpSigners), r.GetWarpComputeUnitsPerSigner())
	}
	if t.Action.OutputsWarpMessage() {
		// Chunks later accounted for by call to [StateKeys]
		computeUnitsOp.Add(r.GetOutgoingWarpComputeUnits())
	}
	computeUnits, err := computeUnitsOp.Value()
	if err != nil {
		return Dimensions{}, err
	}

	// Calculate storage cost
	stateKeys, err := t.StateKeys(sm)
	if err != nil {
		return Dimensions{}, err
	}
	readsOp := math.NewUint64Operator(0)
	allocatesOp := math.NewUint64Operator(0)
	writesOp := math.NewUint64Operator(0)
	for k, perms := range stateKeys {
		// Compute value costs
		maxChunks, ok := keys.MaxChunks(k)
		if !ok {
			return Dimensions{}, ErrInvalidKeyValue
		}

		// Compute key costs
		if perms.Has(state.Read) {
			readsOp.Add(r.GetStorageKeyReadUnits())
			readsOp.MulAdd(uint64(maxChunks), r.GetStorageValueReadUnits())
		}
		if perms.Has(state.Allocate) {
			allocatesOp.Add(r.GetStorageKeyAllocateUnits())
			allocatesOp.MulAdd(uint64(maxChunks), r.GetStorageValueAllocateUnits())
		}
		if perms.Has(state.Write) {
			writesOp.Add(r.GetStorageKeyWriteUnits())
			writesOp.MulAdd(uint64(maxChunks), r.GetStorageValueWriteUnits())
		}
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
	return Dimensions{uint64(t.Size()), computeUnits, reads, allocates, writes}, nil
}

// EstimateUnits provides a estimate of the cost to execute a transaction. This is
// typically used during transaction construction.
func EstimateUnits(r Rules, action Action, authFactory AuthFactory, warpMessage *warp.Message) (Dimensions, error) {
	// Estimate bandwidth cost
	authBandwidth, authCompute := authFactory.MaxUnits()
	bandwidth := BaseSize + consts.ByteLen + uint64(action.Size()) + consts.ByteLen + authBandwidth

	actionStateKeyChunks := action.StateKeyChunks()
	sponsorStateKeyChunks := r.GetSponsorStateKeyChunks()
	stateKeyChunks := make([]uint16, 0, len(sponsorStateKeyChunks)+len(actionStateKeyChunks))
	stateKeyChunks = append(stateKeyChunks, sponsorStateKeyChunks...)
	stateKeyChunks = append(stateKeyChunks, actionStateKeyChunks...)

	// Estimate compute costs
	computeUnitsOp := math.NewUint64Operator(r.GetBaseComputeUnits())
	computeUnitsOp.Add(authCompute)
	computeUnitsOp.Add(action.ComputeUnits(r))
	if warpMessage != nil {
		bandwidth += uint64(codec.BytesLen(warpMessage.Bytes()))
		stateKeyChunks = append(stateKeyChunks, MaxIncomingWarpChunks)
		computeUnitsOp.Add(r.GetBaseWarpComputeUnits())
		numSigners, err := warpMessage.Signature.NumSigners()
		if err != nil {
			return Dimensions{}, err
		}
		computeUnitsOp.MulAdd(uint64(numSigners), r.GetWarpComputeUnitsPerSigner())
	}
	if action.OutputsWarpMessage() {
		stateKeyChunks = append(stateKeyChunks, MaxOutgoingWarpChunks)
		computeUnitsOp.Add(r.GetOutgoingWarpComputeUnits())
	}
	computeUnits, err := computeUnitsOp.Value()
	if err != nil {
		return Dimensions{}, err
	}

	// Estimate storage costs
	//
	// We don't know if these keys are reads, writes, or allocates, so we
	// assume they do all.
	//
	// TODO: make this bound tighter
	readsOp := math.NewUint64Operator(0)
	allocatesOp := math.NewUint64Operator(0)
	writesOp := math.NewUint64Operator(0)
	for maxChunks := range stateKeyChunks {
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

// TODO: remove balance check from this function
// to enable more granular handling of balance check
// failures
func (t *Transaction) SyntacticVerify(
	ctx context.Context,
	s StateManager,
	r Rules,
	timestamp int64,
) (Dimensions, error) {
	if err := t.Base.Execute(r.ChainID(), r, timestamp); err != nil { // enforces valid partition size
		return Dimensions{}, err
	}
	start, end := t.Action.ValidRange(r)
	if start >= 0 && timestamp < start {
		return Dimensions{}, ErrActionNotActivated
	}
	if end >= 0 && timestamp > end {
		return Dimensions{}, ErrActionNotActivated
	}
	start, end = t.Auth.ValidRange(r)
	if start >= 0 && timestamp < start {
		return Dimensions{}, ErrAuthNotActivated
	}
	if end >= 0 && timestamp > end {
		return Dimensions{}, ErrAuthNotActivated
	}
	return t.Units(s, r)
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
	s StateManager,
	r Rules,
	ts *tstate.TStateView,
	timestamp int64,
	warpVerified bool,
) (*Result, error) {
	// Always charge fee first (in case [Action] moves funds)
	units, err := t.Units(s, r)
	if err != nil {
		// Should never happen
		return nil, err
	}
	fee, err := MulSum(r.GetUnitPrices(), units)
	if err != nil {
		// Should never happen
		return nil, err
	}
	// @todo currently there are no bonds implemented?
	if err := s.Deduct(ctx, t.Auth.Sponsor(), ts, fee); err != nil {
		return nil, err
	}

	// Check warp message is not duplicate
	if t.WarpMessage != nil {
		p := s.IncomingWarpKeyPrefix(t.WarpMessage.SourceChainID, t.warpID)
		k := keys.EncodeChunks(p, MaxIncomingWarpChunks)
		_, err := ts.Get(ctx, k)
		switch {
		case err == nil:
			// Override all errors because warp message is a duplicate
			warpVerified = false
		case errors.Is(err, database.ErrNotFound):
			// This means there are no conflicts
		case err != nil:
			// An error here can indicate there is an issue with the database or that
			// the key was not properly specified.
			return &Result{
				Valid:        true,
				WarpVerified: warpVerified,

				Success: false,
				Output:  utils.ErrBytes(err),

				Consumed:    units,
				Fee:         fee,
				WarpMessage: nil,
			}, nil
		}
	}

	// We create a temp state checkpoint to ensure we don't commit failed actions to state.
	actionStart := ts.OpIndex()
	handleRevert := func(rerr error) (*Result, error) {
		// Be warned that the variables captured in this function
		// are set when this function is defined. If any of them are
		// modified later, they will not be used here.
		ts.Rollback(ctx, actionStart)
		return &Result{
			Valid:        true,
			WarpVerified: warpVerified,

			Success: false,
			Output:  utils.ErrBytes(err),

			Consumed: units,
			Fee:      fee,
		}, nil
	}
	success, output, warpMessage, err := t.Action.Execute(ctx, r, ts, timestamp, t.Auth.Actor(), t.id, warpVerified)
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
			if err := ts.Put(ctx, k, nil); err != nil {
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
			if err := ts.Put(ctx, k, warpMessage.Bytes()); err != nil {
				return handleRevert(err)
			}
		}
	}
	return &Result{
		Valid:        true,
		WarpVerified: warpVerified,

		Success: success,
		Output:  output,

		Consumed: units,
		Fee:      fee,

		WarpMessage: warpMessage,
	}, nil
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
	capacity := min(txCount, initialCapacity)
	txs := make([]*Transaction, 0, capacity) // DoS to set size to txCount
	txsSeen := set.NewSet[ids.ID](capacity)
	for i := 0; i < txCount; i++ {
		tx, err := UnmarshalTx(p, actionRegistry, authRegistry)
		if err != nil {
			return nil, nil, err
		}
		txID := tx.ID()
		if txsSeen.Contains(txID) {
			return nil, nil, ErrDuplicateTx
		}
		txsSeen.Add(tx.ID())
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

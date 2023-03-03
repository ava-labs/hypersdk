// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/mempool"
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

	bytes          []byte
	size           uint64
	id             ids.ID
	numWarpSigners int
	warpID         ids.ID
}

type WarpMessage struct {
	ID        ids.ID
	Message   *warp.Message
	VerifyErr error
}

func NewTx(base *Base, act Action) *Transaction {
	return &Transaction{
		Base:   base,
		Action: act,
	}
}

func NewWarpTx(base *Base, wm *warp.Message, act Action) *Transaction {
	return &Transaction{
		Base:        base,
		WarpMessage: wm,
		Action:      act,
	}
}

func (t *Transaction) Digest() ([]byte, error) {
	p := codec.NewWriter(consts.MaxInt)
	t.Base.Marshal(p)
	var warpBytes []byte
	if t.WarpMessage != nil {
		warpBytes = t.WarpMessage.Bytes()
	}
	p.PackBytes(warpBytes)
	t.Action.Marshal(p)
	return p.Bytes(), p.Err()
}

func (t *Transaction) Sign(factory AuthFactory) error {
	msg, err := t.Digest()
	if err != nil {
		return err
	}
	auth, err := factory.Sign(msg, t.Action)
	if err != nil {
		return err
	}
	t.Auth = auth
	return nil
}

func (t *Transaction) Init(
	_ context.Context,
	actionRegistry ActionRegistry,
	authRegistry AuthRegistry,
) (func() error, error) {
	if len(t.bytes) == 0 {
		p := codec.NewWriter(consts.MaxInt)
		if err := t.Marshal(p, actionRegistry, authRegistry); err != nil {
			// should never happen
			return nil, err
		}
		t.bytes = p.Bytes()
		t.id = utils.ToID(t.bytes)
		t.size = uint64(len(t.bytes))
		if msg := t.WarpMessage; msg != nil {
			numSigners, err := msg.Signature.NumSigners()
			if err != nil {
				return nil, fmt.Errorf("%w: could not calculate number of warp signers", err)
			}
			t.numWarpSigners = numSigners
			t.warpID = utils.ToID(msg.Bytes())
		}
	}

	return func() error {
		// Verify sender
		msg, err := t.Digest()
		if err != nil {
			return err
		}
		return t.Auth.AsyncVerify(msg)
	}, nil
}

func (t *Transaction) Bytes() []byte { return t.bytes } // TODO: when to use vs Marshal?

func (t *Transaction) Size() uint64 { return t.size }

func (t *Transaction) ID() ids.ID { return t.id }

func (t *Transaction) Expiry() int64 { return t.Base.Timestamp }

func (t *Transaction) UnitPrice() uint64 { return t.Base.UnitPrice }

// It is ok to have duplicate ReadKeys...the processor will skip them
//
// TODO: verify the invariant that [t.id] is set by this point
func (t *Transaction) StateKeys(stateMapping StateManager) [][]byte {
	keys := append(t.Action.StateKeys(t.Auth, t.ID()), t.Auth.StateKeys()...)
	if t.WarpMessage != nil {
		keys = append(keys, stateMapping.IncomingWarpKey(t.warpID))
	}
	// Always assume a message could export a warp message
	keys = append(keys, stateMapping.OutgoingWarpKey(t.id))
	return keys
}

// Units is charged whether or not a transaction is successful because state
// lookup is not free.
func (t *Transaction) MaxUnits(r Rules) uint64 {
	txFee := r.GetBaseUnits() + t.Action.MaxUnits(r) + t.Auth.MaxUnits(r)
	if t.WarpMessage != nil {
		txFee += r.GetWarpBaseFee()
		txFee += uint64(t.numWarpSigners) * r.GetWarpFeePerSigner()
	}
	if txFee > 0 {
		return txFee
	}
	return 1
}

// PreExecute must not modify state
func (t *Transaction) PreExecute(
	ctx context.Context,
	ectx *ExecutionContext,
	r Rules,
	db Database,
	timestamp int64,
) error {
	if err := t.Base.Execute(ectx.ChainID, r, timestamp); err != nil {
		return err
	}
	start, end := t.Action.ValidRange(r)
	if start >= 0 && timestamp < start {
		return ErrActionNotActivated
	}
	if end >= 0 && timestamp > end {
		return ErrActionNotActivated
	}
	start, end = t.Auth.ValidRange(r)
	if start >= 0 && timestamp < start {
		return ErrAuthNotActivated
	}
	if end >= 0 && timestamp > end {
		return ErrAuthNotActivated
	}
	unitPrice := t.Base.UnitPrice
	if unitPrice < ectx.NextUnitPrice {
		return ErrInsufficientPrice
	}
	if _, err := t.Auth.Verify(ctx, r, db, t.Action); err != nil {
		return fmt.Errorf("%w: %v", ErrAuthFailed, err) //nolint:errorlint
	}
	return t.Auth.CanDeduct(ctx, db, t.MaxUnits(r)*unitPrice)
}

// Execute after knowing a transaction can pay a fee
func (t *Transaction) Execute(
	ctx context.Context,
	r Rules,
	s StateManager,
	tdb *tstate.TState,
	timestamp int64,
	warpMessage *WarpMessage,
) (*Result, error) {
	// Check warp message is not duplicate
	if t.WarpMessage != nil {
		_, err := tdb.GetValue(ctx, s.IncomingWarpKey(t.warpID))
		switch {
		case err == nil:
			// Override all errors if warp message is a duplicate
			//
			// TODO: consider doing this check before performing signature
			// verification.
			warpMessage.VerifyErr = errors.New("duplicate warp message")
		case errors.Is(err, database.ErrNotFound):
			// This means there are no conflicts
		case err != nil:
			return nil, err
		}
	}

	// Verify auth is correct prior to doing anything
	authUnits, err := t.Auth.Verify(ctx, r, tdb, t.Action)
	if err != nil {
		return nil, err
	}

	// Always charge fee first in case [Action] moves funds
	unitPrice := t.Base.UnitPrice
	maxUnits := t.MaxUnits(r)
	if err := t.Auth.Deduct(ctx, tdb, unitPrice*maxUnits); err != nil {
		// This should never fail for low balance (as we check [CanDeductFee]
		// immediately before.
		return nil, err
	}

	// We create a temp state to ensure we don't commit failed actions to state.
	start := tdb.OpIndex()
	result, err := t.Action.Execute(ctx, r, tdb, timestamp, t.Auth, t.id, warpMessage)
	if err != nil {
		return nil, err
	}
	if len(result.Output) == 0 && result.Output != nil {
		// Enforce object standardization (this is a VM bug and we should fail
		// fast)
		return nil, ErrInvalidObject
	}
	if !result.Success {
		// Only keep changes if successful
		tdb.Rollback(ctx, start)
	}

	// Update action units with other items
	result.Units += r.GetBaseUnits() + authUnits
	if t.WarpMessage != nil {
		result.Units += r.GetWarpBaseFee()
		result.Units += uint64(t.numWarpSigners) * r.GetWarpFeePerSigner()
	}

	// Return any funds from unused units
	refund := (maxUnits - result.Units) * unitPrice
	if refund > 0 {
		if err := t.Auth.Refund(ctx, tdb, refund); err != nil {
			return nil, err
		}
	}

	// Store incoming warp messages in state by their ID to prevent replays
	if t.WarpMessage != nil {
		if err := tdb.Insert(ctx, s.IncomingWarpKey(t.warpID), nil); err != nil {
			return nil, err
		}
	}

	// Store newly created warp messages in state by their txID to ensure we can
	// always sign for a message
	if result.WarpMessage != nil {
		// We use txID here because we will not know the warpID before execution.
		if err := tdb.Insert(ctx, s.OutgoingWarpKey(t.id), result.WarpMessage.Bytes()); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Used by mempool
func (t *Transaction) Payer() string {
	return string(t.Auth.Payer())
}

func (t *Transaction) Marshal(
	p *codec.Packer,
	actionRegistry *codec.TypeParser[Action, *warp.Message, bool],
	authRegistry *codec.TypeParser[Auth, *warp.Message, bool],
) error {
	if len(t.bytes) > 0 {
		p.PackFixedBytes(t.bytes)
		return p.Err()
	}

	actionByte, _, _, ok := actionRegistry.LookupType(t.Action)
	if !ok {
		return fmt.Errorf("unknown action type %T", t.Action)
	}
	authByte, _, _, ok := authRegistry.LookupType(t.Auth)
	if !ok {
		return fmt.Errorf("unknown auth type %T", t.Auth)
	}
	t.Base.Marshal(p)
	var warpBytes []byte
	if t.WarpMessage != nil {
		warpBytes = t.WarpMessage.Bytes()
	}
	p.PackBytes(warpBytes)
	p.PackByte(actionByte)
	t.Action.Marshal(p)
	p.PackByte(authByte)
	t.Auth.Marshal(p)
	return p.Err()
}

func MarshalTxs(
	txs []*Transaction,
	actionRegistry ActionRegistry,
	authRegistry AuthRegistry,
) ([]byte, error) {
	p := codec.NewWriter(NetworkSizeLimit)
	p.PackInt(len(txs))
	for _, tx := range txs {
		if err := tx.Marshal(p, actionRegistry, authRegistry); err != nil {
			return nil, err
		}
	}
	return p.Bytes(), p.Err()
}

func UnmarshalTxs(
	raw []byte,
	maxCount int,
	actionRegistry ActionRegistry,
	authRegistry AuthRegistry,
) ([]*Transaction, error) {
	p := codec.NewReader(raw, NetworkSizeLimit)
	txCount := p.UnpackInt(true)
	if txCount > maxCount {
		return nil, ErrTooManyTxs
	}
	txs := make([]*Transaction, txCount)
	for i := 0; i < txCount; i++ {
		tx, err := UnmarshalTx(p, actionRegistry, authRegistry)
		if err != nil {
			return nil, err
		}
		txs[i] = tx
	}
	if !p.Empty() {
		// Ensure no leftover bytes
		return nil, ErrInvalidObject
	}
	return txs, p.Err()
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
	authType := p.UnpackByte()
	unmarshalAuth, authWarp, ok := authRegistry.LookupIndex(authType)
	if !ok {
		return nil, fmt.Errorf("%w: %d is unknown action type", ErrInvalidObject, actionType)
	}
	if authWarp && warpMessage == nil {
		return nil, fmt.Errorf("%w: auth %d", ErrExpectedWarpMessage, authType)
	}
	auth, err := unmarshalAuth(p, warpMessage)
	if err != nil {
		return nil, fmt.Errorf("%w: could not unmarshal auth", err)
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
	tx.bytes = p.Bytes()[start:p.Offset()] // ensure errors handled before grabbing memory
	tx.size = uint64(len(tx.bytes))
	tx.id = utils.ToID(tx.bytes)
	if tx.WarpMessage != nil {
		tx.numWarpSigners = numWarpSigners
		tx.warpID = utils.ToID(warpBytes)
	}
	return &tx, nil
}

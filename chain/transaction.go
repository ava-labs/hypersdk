// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"fmt"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/internal/math"
)

type Transaction struct {
	Base *Base `json:"base"`

	Actions Actions `json:"actions"`

	bytes []byte
	size  int
}

func NewTx(base *Base, actions Actions) *Transaction {
	return &Transaction{
		Base:    base,
		Actions: actions,
	}
}

// Bytes returns the byte slice representation of the tx
func (t *Transaction) Bytes() ([]byte, error) {
	if len(t.bytes) > 0 {
		return t.bytes, nil
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
	t.bytes = p.Bytes()
	return t.bytes, p.Err()
}

// Sign returns a new signed transaction with the unsigned tx copied from
// the original and a signature provided by the authFactory
func (t *Transaction) Sign(
	factory AuthFactory,
	actionRegistry ActionRegistry,
	authRegistry AuthRegistry,
) (*SignedTransaction, error) {
	msg, err := t.Bytes()
	if err != nil {
		return nil, err
	}
	auth, err := factory.Sign(msg)
	if err != nil {
		return nil, err
	}

	signedTransaction := SignedTransaction{
		Transaction: Transaction{
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
	return UnmarshalSignedTx(p, actionRegistry, authRegistry)
}

func (t *Transaction) Size() int { return t.size }

func (t *Transaction) Expiry() int64 { return t.Base.Timestamp }

func (t *Transaction) MaxFee() uint64 { return t.Base.MaxFee }

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

func (t *Transaction) Marshal(p *codec.Packer) error {
	if len(t.bytes) > 0 {
		p.PackFixedBytes(t.bytes)
		return p.Err()
	}
	return t.marshal(p)
}

func (t *Transaction) marshal(p *codec.Packer) error {
	t.Base.Marshal(p)
	if err := p.Err(); err != nil {
		return err
	}

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

func UnmarshalTx(
	p *codec.Packer,
	actionRegistry *codec.TypeParser[Action],
) (*Transaction, error) {
	start := p.Offset()
	base, err := UnmarshalBase(p)
	if err != nil {
		return nil, fmt.Errorf("%w: could not unmarshal base", err)
	}
	actions, err := UnmarshalActions(p, actionRegistry)
	if err != nil {
		return nil, fmt.Errorf("%w: could not unmarshal actions", err)
	}

	var tx Transaction
	tx.Base = base
	tx.Actions = actions
	if err := p.Err(); err != nil {
		return nil, p.Err()
	}
	codecBytes := p.Bytes()
	tx.bytes = codecBytes[start:p.Offset()] // ensure errors handled before grabbing memory
	tx.size = len(tx.bytes)
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

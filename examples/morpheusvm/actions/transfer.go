// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"

	mconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
)

const (
	TransferComputeUnits = 1
	MaxMemoSize          = 256
	MaxTransferSize      = 1024
)

var (
	ErrOutputValueZero                 = errors.New("value is zero")
	ErrOutputMemoTooLarge              = errors.New("memo is too large")
	_                     chain.Action = (*Transfer)(nil)
)

type Transfer struct {
	// To is the recipient of the [Value].
	To codec.Address `serialize:"true" json:"to"`

	// Amount are transferred to [To].
	Value uint64 `serialize:"true" json:"value"`

	// Optional message to accompany transaction.
	Memo []byte `serialize:"true" json:"memo"`
}

func (*Transfer) GetTypeID() uint8 {
	return mconsts.TransferID
}

func (t *Transfer) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.BalanceKey(actor)): state.Read | state.Write,
		string(storage.BalanceKey(t.To)):  state.All,
	}
}

func (t *Transfer) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, 256),
		MaxSize: MaxTransferSize,
	}
	p.PackByte(mconsts.TransferID)
	// TODO: switch to using Canoto after it supports
	// dynamically loading ABI for CLI support.
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalTransfer(p *codec.Packer) (chain.Action, error) {
	t := &Transfer{}
	if err := codec.LinearCodec.UnmarshalFrom(
		p.Packer,
		t,
	); err != nil {
		return nil, err
	}
	// Ensure that any parsed Transfer instance is valid
	// and below MaxTransferSize
	if len(t.Memo) > MaxMemoSize {
		return nil, ErrOutputMemoTooLarge
	}
	return t, nil
}

func (t *Transfer) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([]byte, error) {
	if t.Value == 0 {
		return nil, ErrOutputValueZero
	}
	if len(t.Memo) > MaxMemoSize {
		return nil, ErrOutputMemoTooLarge
	}
	senderBalance, err := storage.SubBalance(ctx, mu, actor, t.Value)
	if err != nil {
		return nil, err
	}
	receiverBalance, err := storage.AddBalance(ctx, mu, t.To, t.Value)
	if err != nil {
		return nil, err
	}

	result := &TransferResult{
		SenderBalance:   senderBalance,
		ReceiverBalance: receiverBalance,
	}
	return result.Bytes(), nil
}

func (*Transfer) ComputeUnits(chain.Rules) uint64 {
	return TransferComputeUnits
}

func (*Transfer) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

var _ codec.Typed = (*TransferResult)(nil)

type TransferResult struct {
	SenderBalance   uint64 `serialize:"true" json:"sender_balance"`
	ReceiverBalance uint64 `serialize:"true" json:"receiver_balance"`
}

func (*TransferResult) GetTypeID() uint8 {
	return mconsts.TransferID // Common practice is to use the action ID
}

func (t *TransferResult) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, 256),
		MaxSize: MaxTransferSize,
	}
	// TODO: switch to using canoto after dynamic ABI support
	// so that we don't need to ignore the error here.
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalTransferResult(p *codec.Packer) (codec.Typed, error) {
	t := &TransferResult{}
	if err := codec.LinearCodec.UnmarshalFrom(
		p.Packer,
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
}

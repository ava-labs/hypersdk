// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

//go:generate go run github.com/StephenButtolph/canoto/canoto $GOFILE

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"

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
	ErrOutputValueZero                     = errors.New("value is zero")
	ErrOutputMemoTooLarge                  = errors.New("memo is too large")
	ErrUnmarshalEmptyTransfer              = errors.New("cannot unmarshal empty bytes as transfer")
	_                         chain.Action = (*Transfer)(nil)
)

type Transfer struct {
	// To is the recipient of the [Value].
	To codec.Address `canoto:"fixed bytes,1" serialize:"true" json:"to"`

	// Amount are transferred to [To].
	Value uint64 `canoto:"uint,2" serialize:"true" json:"value"`

	// Optional message to accompany transaction.
	Memo []byte `canoto:"bytes,3" serialize:"true" json:"memo"`

	canotoData canotoData_Transfer
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
	return append([]byte{mconsts.TransferID}, t.MarshalCanoto()...)
}

func UnmarshalTransfer(bytes []byte) (chain.Action, error) {
	t := &Transfer{}
	if err := t.UnmarshalCanoto(bytes[1:]); err != nil {
		return nil, err
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
	SenderBalance   uint64 `canoto:"uint,1" serialize:"true" json:"sender_balance"`
	ReceiverBalance uint64 `canoto:"uint,2" serialize:"true" json:"receiver_balance"`

	canotoData canotoData_TransferResult
}

func (*TransferResult) GetTypeID() uint8 {
	return mconsts.TransferID // Common practice is to use the action ID
}

func (t *TransferResult) Bytes() []byte {
	return append([]byte{mconsts.TransferID}, t.MarshalCanoto()...)
}

func UnmarshalTransferResult(b []byte) (codec.Typed, error) {
	t := &TransferResult{}
	if err := t.UnmarshalCanoto(b[1:]); err != nil {
		return nil, err
	}
	return t, nil
}

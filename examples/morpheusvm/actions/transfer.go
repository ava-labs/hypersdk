// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"

	mconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
)

const (
	TransferComputeUnits = 1
	MaxMemoSize          = 256
)

var (
	ErrOutputValueZero                                    = errors.New("value is zero")
	ErrOutputMemoTooLarge                                 = errors.New("memo is too large")
	_                     chain.Action[chain.PendingView] = (*Transfer[chain.PendingView])(nil)
)

type Transfer[T chain.PendingView] struct {
	// To is the recipient of the [Value].
	To codec.Address `serialize:"true" json:"to"`

	// Amount are transferred to [To].
	Value uint64 `serialize:"true" json:"value"`

	// Optional message to accompany transaction.
	Memo codec.Bytes `serialize:"true" json:"memo"`
}

func (*Transfer[_]) GetTypeID() uint8 {
	return mconsts.TransferID
}

func (t *Transfer[_]) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.BalanceKey(actor)): state.Read | state.Write,
		string(storage.BalanceKey(t.To)):  state.All,
	}
}

func (*Transfer[_]) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.BalanceChunks, storage.BalanceChunks}
}

func (t *Transfer[T]) Execute(
	ctx context.Context,
	_ chain.Rules,
	view T,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) (codec.Typed, error) {
	if t.Value == 0 {
		return nil, ErrOutputValueZero
	}
	if len(t.Memo) > MaxMemoSize {
		return nil, ErrOutputMemoTooLarge
	}
	senderBalance, err := storage.SubBalance(ctx, view, actor, t.Value)
	if err != nil {
		return nil, err
	}
	receiverBalance, err := storage.AddBalance(ctx, view, t.To, t.Value, true)
	if err != nil {
		return nil, err
	}

	return &TransferResult{
		SenderBalance:   senderBalance,
		ReceiverBalance: receiverBalance,
	}, nil
}

func (*Transfer[_]) ComputeUnits(chain.Rules) uint64 {
	return TransferComputeUnits
}

func (*Transfer[_]) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

// Implementing chain.Marshaler is optional but can be used to optimize performance when hitting TPS limits
var _ chain.Marshaler = (*Transfer[chain.PendingView])(nil)

func (t *Transfer[_]) Size() int {
	return codec.AddressLen + consts.Uint64Len + codec.BytesLen(t.Memo)
}

func (t *Transfer[_]) Marshal(p *codec.Packer) {
	p.PackAddress(t.To)
	p.PackLong(t.Value)
	p.PackBytes(t.Memo)
}

func UnmarshalTransfer(p *codec.Packer) (chain.Action[*tstate.TStateView], error) {
	var transfer Transfer[*tstate.TStateView]
	p.UnpackAddress(&transfer.To)
	transfer.Value = p.UnpackUint64(true)
	p.UnpackBytes(MaxMemoSize, false, (*[]byte)(&transfer.Memo))
	return &transfer, p.Err()
}

var _ codec.Typed = (*TransferResult)(nil)

type TransferResult struct {
	SenderBalance   uint64 `serialize:"true" json:"sender_balance"`
	ReceiverBalance uint64 `serialize:"true" json:"receiver_balance"`
}

func (*TransferResult) GetTypeID() uint8 {
	return mconsts.TransferID // Common practice is to use the action ID
}

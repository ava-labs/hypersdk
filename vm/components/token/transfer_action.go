// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package token

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/vm/components"
)

const (
	TransferComputeUnits = 1
	MaxMemoSize          = 256
)

var (
	_ 				   components.ActionComponent = (*Transfer)(nil)
	ErrOutputValueZero                 = errors.New("value is zero")
	ErrOutputMemoTooLarge              = errors.New("memo is too large")
)

type Transfer struct {
	// name of the token being transferred
	Name string `serialize:"true" json:"name"`

	// To is the recipient of the [Value].
	To codec.Address `serialize:"true" json:"to"`

	// Amount are transferred to [To].
	Value uint64 `serialize:"true" json:"value"`

	// Optional message to accompany transaction.
	Memo []byte `serialize:"true" json:"memo"`
}

func (t *Transfer) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	bh := NewBalanceHandler([]byte(t.Name))
	return state.Keys{
		string(bh.BalanceKey(actor)): state.Read | state.Write,
		string(bh.BalanceKey(t.To)):  state.All,
	}
}

func (t *Transfer) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) (interface{}, error) {
	bh := NewBalanceHandler([]byte(t.Name))

	if t.Value == 0 {
		return nil, ErrOutputValueZero
	}
	if len(t.Memo) > MaxMemoSize {
		return nil, ErrOutputMemoTooLarge
	}
	senderBalance, err := bh.SubBalance(ctx, mu, actor, t.Value)
	if err != nil {
		return nil, err
	}
	err = bh.AddBalance(ctx, t.To, mu, t.Value, true)
	if err != nil {
		return nil, err
	}

	receiverBalance, err := bh.GetBalance(ctx, mu, t.To)
	if err != nil {
		return nil, err
	}
	
	return &TransferResult{
		SenderBalance:   senderBalance,
		ReceiverBalance: receiverBalance,
	}, nil
}

func (*Transfer) ComputeUnits(chain.Rules) uint64 {
	return TransferComputeUnits
}

func (*Transfer) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

type TransferResult struct {
	SenderBalance   uint64 `serialize:"true" json:"sender_balance"`
	ReceiverBalance uint64 `serialize:"true" json:"receiver_balance"`
}

func NewTransferResult(senderBalance, receiverBalance uint64) *TransferResult {
return &TransferResult{
		SenderBalance:   senderBalance,
		ReceiverBalance: receiverBalance,
	}
}

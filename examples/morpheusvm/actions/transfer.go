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

	mconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
)

const (
	TransferComputeUnits = 1
	MaxMemoSize          = 256
)

var (
	ErrOutputValueZero                 = errors.New("value is zero")
	ErrOutputMemoTooLarge              = errors.New("memo is too large")
	_                     chain.Action = (*Transfer)(nil)
)

type Transfer struct {
	// To is the recipient of the [Value].
	To codec.Address `json:"to"`

	// Amount are transferred to [To].
	Value uint64 `json:"value"`

	// Optional message to accompany transaction.
	Memo []byte `json:"memo"`
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

func (*Transfer) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.BalanceChunks, storage.BalanceChunks}
}

func (t *Transfer) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
) ([][]byte, error) {
	if t.Value == 0 {
		return nil, ErrOutputValueZero
	}
	if len(t.Memo) > MaxMemoSize {
		return nil, ErrOutputMemoTooLarge
	}
	if err := storage.SubBalance(ctx, mu, actor, t.Value); err != nil {
		return nil, err
	}
	if err := storage.AddBalance(ctx, mu, t.To, t.Value, true); err != nil {
		return nil, err
	}
	return nil, nil
}

func (*Transfer) ComputeUnits(chain.Rules) uint64 {
	return TransferComputeUnits
}

func (t *Transfer) Size() int {
	return codec.AddressLen + consts.Uint64Len + codec.BytesLen(t.Memo)
}

// Optional: Use only if you need to manually marshal the action.

// var _ chain.Marshaller = (*Transfer)(nil)

// func (t *Transfer) Marshal(p *codec.Packer) {
// 	p.PackAddress(t.To)
// 	p.PackUint64(t.Value)
// 	p.PackBytes(t.Memo)
// }

func UnmarshalTransfer(p *codec.Packer) (chain.Action, error) {
	var transfer Transfer
	err := codec.AutoUnmarshalStruct(p, &transfer)
	return &transfer, err
}

func (*Transfer) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

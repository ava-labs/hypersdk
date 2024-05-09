// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*Transfer)(nil)

type Transfer struct {
	// To is the recipient of the [Value].
	To codec.Address `json:"to"`

	// Asset to transfer to [To].
	Asset codec.LID `json:"asset"`

	// Amount are transferred to [To].
	Value uint64 `json:"value"`

	// Optional message to accompany transaction.
	Memo []byte `json:"memo"`
}

func (*Transfer) GetTypeID() uint8 {
	return transferID
}

func (t *Transfer) StateKeys(actor codec.Address, _ codec.LID) state.Keys {
	return state.Keys{
		string(storage.BalanceKey(actor, t.Asset)): state.Read | state.Write,
		string(storage.BalanceKey(t.To, t.Asset)):  state.All,
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
	_ codec.LID,
) (bool, uint64, [][]byte) {
	if t.Value == 0 {
		return false, TransferComputeUnits, [][]byte{OutputValueZero}
	}
	if len(t.Memo) > MaxMemoSize {
		return false, CreateAssetComputeUnits, [][]byte{OutputMemoTooLarge}
	}
	if err := storage.SubBalance(ctx, mu, actor, t.Asset, t.Value); err != nil {
		return false, TransferComputeUnits, [][]byte{utils.ErrBytes(err)}
	}
	// TODO: allow sender to configure whether they will pay to create
	if err := storage.AddBalance(ctx, mu, t.To, t.Asset, t.Value, true); err != nil {
		return false, TransferComputeUnits, [][]byte{utils.ErrBytes(err)}
	}
	return true, TransferComputeUnits, [][]byte{{}}
}

func (*Transfer) MaxComputeUnits(chain.Rules) uint64 {
	return TransferComputeUnits
}

func (t *Transfer) Size() int {
	return codec.AddressLen + codec.LIDLen + consts.Uint64Len + codec.BytesLen(t.Memo)
}

func (t *Transfer) Marshal(p *codec.Packer) {
	p.PackLID(t.To)
	p.PackLID(t.Asset)
	p.PackUint64(t.Value)
	p.PackBytes(t.Memo)
}

func UnmarshalTransfer(p *codec.Packer) (chain.Action, error) {
	var transfer Transfer
	p.UnpackLID(true, &transfer.To)
	p.UnpackLID(false, &transfer.Asset) // empty ID is the native asset
	transfer.Value = p.UnpackUint64(true)
	p.UnpackBytes(MaxMemoSize, false, &transfer.Memo)
	return &transfer, p.Err()
}

func (*Transfer) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

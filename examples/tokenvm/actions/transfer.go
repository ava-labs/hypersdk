// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
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
	Asset ids.ID `json:"asset"`

	// Amount are transferred to [To].
	Value uint64 `json:"value"`

	// Optional message to accompany transaction.
	Memo []byte `json:"memo"`
}

func (*Transfer) GetTypeID() uint8 {
	return transferID
}

func (t *Transfer) StateKeys(auth chain.Auth, _ ids.ID) []string {
	return []string{
		string(storage.BalanceKey(auth.Actor(), t.Asset)),
		string(storage.BalanceKey(t.To, t.Asset)),
	}
}

func (*Transfer) StateKeysMaxChunks() []uint16 {
	return []uint16{storage.BalanceChunks, storage.BalanceChunks}
}

func (*Transfer) OutputsWarpMessage() bool {
	return false
}

func (t *Transfer) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	auth chain.Auth,
	_ ids.ID,
	_ bool,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	if t.Value == 0 {
		return false, TransferComputeUnits, OutputValueZero, nil, nil
	}
	if len(t.Memo) > MaxMemoSize {
		return false, CreateAssetComputeUnits, OutputMemoTooLarge, nil, nil
	}
	if err := storage.SubBalance(ctx, mu, auth.Actor(), t.Asset, t.Value); err != nil {
		return false, TransferComputeUnits, utils.ErrBytes(err), nil, nil
	}
	// TODO: allow sender to configure whether they will pay to create
	if err := storage.AddBalance(ctx, mu, t.To, t.Asset, t.Value, true); err != nil {
		return false, TransferComputeUnits, utils.ErrBytes(err), nil, nil
	}
	return true, TransferComputeUnits, nil, nil, nil
}

func (*Transfer) MaxComputeUnits(chain.Rules) uint64 {
	return TransferComputeUnits
}

func (t *Transfer) Size() int {
	return codec.AddressLen + consts.IDLen + consts.Uint64Len + codec.BytesLen(t.Memo)
}

func (t *Transfer) Marshal(p *codec.Packer) {
	p.PackAddress(t.To)
	p.PackID(t.Asset)
	p.PackUint64(t.Value)
	p.PackBytes(t.Memo)
}

func UnmarshalTransfer(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var transfer Transfer
	p.UnpackAddress(&transfer.To)
	p.UnpackID(false, &transfer.Asset) // empty ID is the native asset
	transfer.Value = p.UnpackUint64(true)
	p.UnpackBytes(MaxMemoSize, false, &transfer.Memo)
	return &transfer, p.Err()
}

func (*Transfer) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

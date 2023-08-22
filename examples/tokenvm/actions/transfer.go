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
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*Transfer)(nil)

type Transfer struct {
	// To is the recipient of the [Value].
	To ed25519.PublicKey `json:"to"`

	// Asset to transfer to [To].
	Asset ids.ID

	// Amount are transferred to [To].
	Value uint64 `json:"value"`
}

func (*Transfer) GetTypeID() uint8 {
	return transferID
}

func (t *Transfer) StateKeys(rauth chain.Auth, _ ids.ID) []string {
	return []string{
		string(storage.BalanceKey(auth.GetActor(rauth), t.Asset)),
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
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
	_ bool,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	actor := auth.GetActor(rauth)
	if t.Value == 0 {
		return false, TransferComputeUnits, OutputValueZero, nil, nil
	}
	if err := storage.SubBalance(ctx, db, actor, t.Asset, t.Value); err != nil {
		return false, TransferComputeUnits, utils.ErrBytes(err), nil, nil
	}
	if err := storage.AddBalance(ctx, db, t.To, t.Asset, t.Value); err != nil {
		return false, TransferComputeUnits, utils.ErrBytes(err), nil, nil
	}
	return true, TransferComputeUnits, nil, nil, nil
}

func (*Transfer) MaxComputeUnits(chain.Rules) uint64 {
	return TransferComputeUnits
}

func (*Transfer) Size() int {
	return ed25519.PublicKeyLen + consts.IDLen + consts.Uint64Len
}

func (t *Transfer) Marshal(p *codec.Packer) {
	p.PackPublicKey(t.To)
	p.PackID(t.Asset)
	p.PackUint64(t.Value)
}

func UnmarshalTransfer(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var transfer Transfer
	p.UnpackPublicKey(false, &transfer.To) // can transfer to blackhole
	p.UnpackID(false, &transfer.Asset)     // empty ID is the native asset
	transfer.Value = p.UnpackUint64(true)
	return &transfer, p.Err()
}

func (*Transfer) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

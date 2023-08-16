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
	"github.com/ava-labs/hypersdk/examples/morpheusvm/auth"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*Transfer)(nil)

type Transfer struct {
	// To is the recipient of the [Value].
	To ed25519.PublicKey `json:"to"`

	// Amount are transferred to [To].
	Value uint64 `json:"value"`
}

func (*Transfer) GetTypeID() uint8 {
	return transferID
}

func (t *Transfer) StateKeys(rauth chain.Auth, _ ids.ID) [][]byte {
	return [][]byte{
		storage.PrefixBalanceKey(auth.GetActor(rauth)),
		storage.PrefixBalanceKey(t.To),
	}
}

func (t *Transfer) StateKeysCount() int {
	return 2
}

func (t *Transfer) OutputsWarpMessage() bool {
	return false
}

func (t *Transfer) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
	_ bool,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	actor := auth.GetActor(rauth)
	if t.Value == 0 {
		return false, 1, OutputValueZero, nil, nil
	}
	if err := storage.SubBalance(ctx, db, actor, t.Value); err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}
	if err := storage.AddBalance(ctx, db, t.To, t.Value); err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}
	return true, 1, nil, nil, nil
}

func (t *Transfer) MaxComputeUnits(chain.Rules) uint64 {
	return 1
}

func (*Transfer) Size() int {
	return ed25519.PublicKeyLen + consts.Uint64Len
}

func (t *Transfer) Marshal(p *codec.Packer) {
	p.PackPublicKey(t.To)
	p.PackUint64(t.Value)
}

func UnmarshalTransfer(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var transfer Transfer
	p.UnpackPublicKey(false, &transfer.To) // can transfer to blackhole
	transfer.Value = p.UnpackUint64(true)
	return &transfer, p.Err()
}

func (*Transfer) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

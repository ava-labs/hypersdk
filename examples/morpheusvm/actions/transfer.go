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
	"github.com/ava-labs/hypersdk/examples/morpheusvm/auth"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*Transfer)(nil)

type Transfer struct {
	// To is the recipient of the [Value].
	To codec.ShortBytes `json:"to"`

	// Amount are transferred to [To].
	Value uint64 `json:"value"`
}

func (*Transfer) GetTypeID() uint8 {
	return transferID
}

func (t *Transfer) StateKeys(rauth chain.Auth, _ ids.ID) []string {
	return []string{
		string(storage.BalanceKey(auth.GetSigner(rauth))),
		string(storage.BalanceKey(t.To)),
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
	rauth chain.Auth,
	_ ids.ID,
	_ bool,
) (bool, uint64, []byte, *warp.UnsignedMessage, error) {
	signer := auth.GetSigner(rauth)
	if t.Value == 0 {
		return false, 1, OutputValueZero, nil, nil
	}
	if err := storage.SubBalance(ctx, mu, signer, t.Value); err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}
	if err := storage.AddBalance(ctx, mu, t.To, t.Value, true); err != nil {
		return false, 1, utils.ErrBytes(err), nil, nil
	}
	return true, 1, nil, nil, nil
}

func (*Transfer) MaxComputeUnits(chain.Rules) uint64 {
	return TransferComputeUnits
}

func (t *Transfer) Size() int {
	return codec.ShortBytesLen(t.To) + consts.Uint64Len
}

func (t *Transfer) Marshal(p *codec.Packer) {
	p.PackShortBytes(t.To)
	p.PackUint64(t.Value)
}

func UnmarshalTransfer(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	// Unpack bytes
	var transfer Transfer
	p.UnpackShortBytes(&transfer.To)
	transfer.Value = p.UnpackUint64(true)
	if err := p.Err(); err != nil {
		return nil, err
	}

	// Ensure address is well-formatted
	if !auth.VerifyAccountFormat(transfer.To) {
		return nil, auth.ErrMalformedAccount
	}
	return &transfer, nil
}

func (*Transfer) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

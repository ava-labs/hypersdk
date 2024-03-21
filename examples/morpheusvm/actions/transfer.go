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
	mconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Action = (*Transfer)(nil)

type Transfer struct {
	// To is the recipient of the [Value].
	To codec.Address `json:"to"`

	// Allocate is whether or not to create [To], if they don't exist.
	Create bool `json:"create"`

	// Amount are transferred to [To].
	Value uint64 `json:"value"`

	// Memo is an optional field that can be used to store arbitrary data.
	Memo []byte `json:"memo"`
}

func (*Transfer) GetTypeID() uint8 {
	return mconsts.TransferID
}

func (t *Transfer) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	if !t.Create {
		return state.Keys{
			string(storage.BalanceKey(actor)): state.Read | state.Write,
			string(storage.BalanceKey(t.To)):  state.Read | state.Write,
		}
	}
	return state.Keys{
		string(storage.BalanceKey(actor)): state.Read | state.Write,
		string(storage.BalanceKey(t.To)):  state.All,
	}
}

func (*Transfer) StateKeyChunks() []uint16 {
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
	actor codec.Address,
	_ ids.ID,
	_ bool,
) (bool, []byte, *warp.UnsignedMessage, error) {
	if t.Value == 0 {
		return false, OutputValueZero, nil, nil
	}
	if err := storage.SubBalance(ctx, mu, actor, t.Value); err != nil {
		return false, utils.ErrBytes(err), nil, nil
	}
	if err := storage.AddBalance(ctx, mu, t.To, t.Value); err != nil {
		return false, utils.ErrBytes(err), nil, nil
	}
	return true, nil, nil, nil
}

func (*Transfer) ComputeUnits(chain.Rules) uint64 {
	return TransferComputeUnits
}

func (t *Transfer) Size() int {
	return codec.AddressLen + consts.BoolLen + consts.Uint64Len + codec.BytesLen(t.Memo)
}

func (t *Transfer) Marshal(p *codec.Packer) {
	p.PackAddress(t.To)
	p.PackBool(t.Create)
	p.PackUint64(t.Value)
	p.PackBytes(t.Memo)
}

func UnmarshalTransfer(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var transfer Transfer
	p.UnpackAddress(&transfer.To) // we do not verify the typeID is valid
	transfer.Create = p.UnpackBool()
	transfer.Value = p.UnpackUint64(true)
	p.UnpackBytes(consts.NetworkSizeLimit, false, &transfer.Memo)
	if err := p.Err(); err != nil {
		return nil, err
	}
	return &transfer, nil
}

func (*Transfer) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

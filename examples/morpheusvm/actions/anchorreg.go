// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"bytes"
	"context"
	"slices"

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

var _ chain.Action = (*AnchorRegister)(nil)

type AnchorRegister struct {
	Url       string `json:"url"`
	Namespace []byte `json:"namespace"`
	Delete    bool   `json:"delete"`
}

func (*AnchorRegister) GetTypeID() uint8 {
	return mconsts.AnchorRegisterID
}

func (t *AnchorRegister) StateKeys(actor codec.Address, _ ids.ID) state.Keys {
	return state.Keys{
		string(storage.AnchorKey(t.Namespace)): state.All,
		string(storage.AnchorRegistryKey()):    state.All,
	}
}

func (*AnchorRegister) StateKeyChunks() []uint16 {
	return []uint16{storage.BalanceChunks, storage.BalanceChunks}
}

func (*AnchorRegister) OutputsWarpMessage() bool {
	return false
}

func (t *AnchorRegister) Execute(
	ctx context.Context,
	_ chain.Rules,
	mu state.Mutable,
	_ int64,
	actor codec.Address,
	_ ids.ID,
	_ bool,
) (bool, []byte, *warp.UnsignedMessage, error) {
	namespaces, _, err := storage.GetAnchors(ctx, mu)
	if err != nil {
		return false, utils.ErrBytes(err), nil, nil
	}

	if t.Delete {
		nsIdx := -1
		for i, ns := range namespaces {
			if bytes.Equal(t.Namespace, ns) {
				nsIdx = i
				break
			}
		}
		namespaces = slices.Delete(namespaces, nsIdx, nsIdx+1)
		if err := storage.DelAnchor(ctx, mu, t.Namespace); err != nil {
			return false, utils.ErrBytes(err), nil, nil
		}
	} else {
		namespaces = append(namespaces, t.Namespace)
		if err := storage.SetAnchor(ctx, mu, t.Namespace, t.Url); err != nil {
			return false, utils.ErrBytes(err), nil, nil
		}
	}
	if err := storage.SetAnchors(ctx, mu, namespaces); err != nil {
		return false, utils.ErrBytes(err), nil, nil
	}
	return true, nil, nil, nil
}

func (*AnchorRegister) ComputeUnits(chain.Rules) uint64 {
	return AnchorRegisterComputeUnits
}

func (t *AnchorRegister) Size() int {
	return codec.BytesLen(t.Namespace) + codec.StringLen(t.Url) + consts.BoolLen
}

func (t *AnchorRegister) Marshal(p *codec.Packer) {
	p.PackBytes(t.Namespace)
	p.PackString(t.Url)
	p.PackBool(t.Delete)
}

func UnmarshalAnchorRegister(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var anchorReg AnchorRegister
	p.UnpackBytes(-1, false, &anchorReg.Namespace)
	anchorReg.Url = p.UnpackString(false)
	anchorReg.Delete = p.UnpackBool()
	return &anchorReg, nil
}

func (*AnchorRegister) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

func (*AnchorRegister) NMTNamespace() []byte {
	return defaultNMTNamespace // TODO: mark this the same to registering namespace?
}

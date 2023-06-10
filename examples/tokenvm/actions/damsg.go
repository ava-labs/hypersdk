// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
)

var _ chain.Action = (*DASequencerMsg)(nil)

type DASequencerMsg struct {
	Data        []byte `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	FromAddress crypto.PublicKey `json:"from_address"`
	// `protobuf:"bytes,3,opt,name=from_address,json=fromAddress,proto3" json:"from_address,omitempty"`
}

func (t *DASequencerMsg) StateKeys(rauth chain.Auth, _ ids.ID) [][]byte {
	// owner, err := utils.ParseAddress(t.FromAddress)
	// if err != nil {
	// 	return nil, err
	// }

	return [][]byte{
		// We always pay fees with the native asset (which is [ids.Empty])
		storage.PrefixBalanceKey(t.FromAddress, ids.Empty),
	}
}

func (t *DASequencerMsg) Execute(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	_ int64,
	rauth chain.Auth,
	_ ids.ID,
	_ bool,
) (*chain.Result, error) {
	unitsUsed := t.MaxUnits(r) // max units == units
	return &chain.Result{Success: true, Units: unitsUsed}, nil
}

func (*DASequencerMsg) MaxUnits(chain.Rules) uint64 {
	// We use size as the price of this transaction but we could just as easily
	// use any other calculation.
	return crypto.PublicKeyLen + crypto.SignatureLen
}

func (t *DASequencerMsg) Marshal(p *codec.Packer) {
	p.PackPublicKey(t.FromAddress)
	p.PackBytes(t.Data)
}

func UnmarshalDASequencerMsg(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var sequencermsg DASequencerMsg
	p.UnpackPublicKey(false, &sequencermsg.FromAddress)
	//TODO need to correct this
	p.UnpackBytes(8, false, &sequencermsg.Data)
	return &sequencermsg, p.Err()
}

func (*DASequencerMsg) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

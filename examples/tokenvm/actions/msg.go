// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"

	"github.com/AnomalyFi/hypersdk/chain"
	"github.com/AnomalyFi/hypersdk/codec"
	"github.com/AnomalyFi/hypersdk/crypto"
	"github.com/AnomalyFi/hypersdk/examples/tokenvm/storage"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
)

var _ chain.Action = (*SequencerMsg)(nil)

type SequencerMsg struct {
	//TODO might need to add this back in at some point but rn it should be fine
	ChainId     []byte           `protobuf:"bytes,1,opt,name=chain_id,json=chainId,proto3" json:"chain_id,omitempty"`
	Data        []byte           `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
	FromAddress crypto.PublicKey `json:"from_address"`
	// `protobuf:"bytes,3,opt,name=from_address,json=fromAddress,proto3" json:"from_address,omitempty"`
}

func (t *SequencerMsg) StateKeys(rauth chain.Auth, _ ids.ID) [][]byte {
	// owner, err := utils.ParseAddress(t.FromAddress)
	// if err != nil {
	// 	return nil, err
	// }

	return [][]byte{
		// We always pay fees with the native asset (which is [ids.Empty])
		storage.PrefixBalanceKey(t.FromAddress, ids.Empty),
	}
}

func (t *SequencerMsg) Execute(
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

func (*SequencerMsg) MaxUnits(chain.Rules) uint64 {
	// We use size as the price of this transaction but we could just as easily
	// use any other calculation.
	return crypto.PublicKeyLen + crypto.SignatureLen
}

func (t *SequencerMsg) Marshal(p *codec.Packer) {
	p.PackPublicKey(t.FromAddress)
	p.PackBytes(t.Data)
	p.PackBytes(t.ChainId)
}

func UnmarshalSequencerMsg(p *codec.Packer, _ *warp.Message) (chain.Action, error) {
	var sequencermsg SequencerMsg
	p.UnpackPublicKey(false, &sequencermsg.FromAddress)
	//TODO need to correct this and check byte count
	p.UnpackBytes(8, false, &sequencermsg.Data)
	p.UnpackBytes(8, false, &sequencermsg.ChainId)
	return &sequencermsg, p.Err()
}

func (*SequencerMsg) ValidRange(chain.Rules) (int64, int64) {
	// Returning -1, -1 means that the action is always valid.
	return -1, -1
}

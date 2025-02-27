// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

var _ chain.Auth = (*TestAuth)(nil)

type TestAuth struct {
	ActorM          codec.Address `serialize:"true" json:"actor"`
	SponsorM        codec.Address `serialize:"true" json:"sponsor"`
	NumComputeUnits uint64        `serialize:"true" json:"computeUnits"`
}

func (ta *TestAuth) ComputeUnits(chain.Rules) uint64 {
	return ta.NumComputeUnits
}

func (*TestAuth) Verify(context.Context, []byte) error {
	return nil
}

func (ta *TestAuth) Actor() codec.Address {
	return ta.ActorM
}

func (ta *TestAuth) Sponsor() codec.Address {
	return ta.SponsorM
}

func (ta *TestAuth) Marshal(p *codec.Packer) {
	_ = codec.LinearCodec.MarshalInto(ta, p.Packer)
}

func (*TestAuth) Size() int {
	return 0
}

func (*TestAuth) GetTypeID() uint8 {
	return 0
}

func (*TestAuth) ValidRange(chain.Rules) (start int64, end int64) {
	return -1, -1
}

func unmarshalTestAuth(p *codec.Packer) (chain.Auth, error) {
	var testAuth TestAuth
	err := codec.LinearCodec.UnmarshalFrom(p.Packer, &testAuth)
	return &testAuth, err
}

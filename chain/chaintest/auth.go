// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"
	"errors"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

var (
	ErrTestAuthVerify            = errors.New("test auth verification error")
	_                 chain.Auth = (*TestAuth)(nil)
)

type TestAuth struct {
	NumComputeUnits uint64        `canoto:"fint64,1" serialize:"true" json:"numComputeUnits"`
	ActorAddress    codec.Address `canoto:"fixed bytes,2" serialize:"true" json:"actor"`
	SponsorAddress  codec.Address `canoto:"fixed bytes,3" serialize:"true" json:"sponsor"`
	ShouldErr       bool          `canoto:"bool,4" serialize:"true" json:"shouldErr"`
	Start           int64         `canoto:"sint,5" serialize:"true" json:"start"`
	End             int64         `canoto:"sint,6" serialize:"true" json:"end"`

	canotoData canotoData_TestAuth
}

func (t *TestAuth) GetTypeID() uint8 {
	return 0
}

func (t *TestAuth) Bytes() []byte {
	return t.MarshalCanoto()
}

// ValidRange returns the start/end fields of the action unless 0 is specified.
// If 0 is specified, return -1 for always valid, which is a more useful default value.
func (t *TestAuth) ValidRange(_ chain.Rules) (int64, int64) {
	start := t.Start
	end := t.End
	if start == 0 {
		start = -1
	}
	if end == 0 {
		end = -1
	}
	return start, end
}

func (t *TestAuth) Marshal(p *codec.Packer) {
	codec.LinearCodec.MarshalInto(t, p.Packer)
}

func (t *TestAuth) Size() int {
	return consts.Uint64Len + 2*codec.AddressLen + consts.BoolLen
}

func UnmarshalAuth(p *codec.Packer) (chain.Auth, error) {
	t := new(TestAuth)

	if err := codec.LinearCodec.UnmarshalFrom(p.Packer, t); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *TestAuth) ComputeUnits(_ chain.Rules) uint64 {
	return t.NumComputeUnits
}

func (t *TestAuth) Actor() codec.Address {
	return t.ActorAddress
}

func (t *TestAuth) Sponsor() codec.Address {
	return t.SponsorAddress
}

func (t *TestAuth) Verify(ctx context.Context, msg []byte) error {
	if t.ShouldErr {
		return ErrTestAuthVerify
	}
	return nil
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

var (
	ErrTestAuthVerify            = errors.New("test auth verification error")
	_                 chain.Auth = (*TestAuth)(nil)
)

type TestAuth struct {
	NumComputeUnits uint64        `serialize:"true" json:"numComputeUnits"`
	ActorAddress    codec.Address `serialize:"true" json:"actor"`
	SponsorAddress  codec.Address `serialize:"true" json:"sponsor"`
	ShouldErr       bool          `serialize:"true" json:"shouldErr"`
	Start           int64         `serialize:"true" json:"start"`
	End             int64         `serialize:"true" json:"end"`
}

func (t *TestAuth) GetTypeID() uint8 {
	return 0
}

func (t *TestAuth) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, 256),
		MaxSize: 256,
	}
	p.PackByte(t.GetTypeID())
	// TODO: switch to using canoto after dynamic ABI support
	// so that we don't need to ignore the error here.
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalTestAuth(p *codec.Packer) (codec.Typed, error) {
	t := &TestAuth{}
	if err := codec.LinearCodec.UnmarshalFrom(
		p.Packer,
		t,
	); err != nil {
		return nil, err
	}
	return t, nil
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

type TestAuthFactory struct {
	TestAuth *TestAuth
}

func (t *TestAuthFactory) Sign(msg []byte) (chain.Auth, error) {
	return t.TestAuth, nil
}

func (t *TestAuthFactory) MaxUnits() (bandwidth uint64, compute uint64) {
	panic("not implemented")
}

func (t *TestAuthFactory) Address() codec.Address {
	panic("not implemented")
}

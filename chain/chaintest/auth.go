// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
)

const TestAuthTypeID = 0

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
	return TestAuthTypeID
}

func (t *TestAuth) Bytes() []byte {
	p := &wrappers.Packer{
		Bytes:   make([]byte, 0, 256),
		MaxSize: 256,
	}
	p.PackByte(t.GetTypeID())
	// XXX: AvalancheGo codec should never error for a valid value. Running e2e, we only
	// interact with values unmarshalled from the network, which should guarantee a valid
	// value here.
	// Panic if we fail to marshal a value here to catch any potential bugs early.
	// TODO: complete migration of user defined types to Canoto, so we do not need a panic
	// here.
	_ = codec.LinearCodec.MarshalInto(t, p)
	return p.Bytes
}

func UnmarshalTestAuth(bytes []byte) (chain.Auth, error) {
	t := &TestAuth{}

	if bytes[0] != TestAuthTypeID {
		return nil, fmt.Errorf("unexpected test auth typeID: %d != %d", bytes[0], TestAuthTypeID)
	}

	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: bytes[1:]},
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

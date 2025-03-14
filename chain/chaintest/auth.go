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
)

const TestAuthTypeID = 0

var (
	ErrTestAuthVerify                   = errors.New("test auth verification error")
	_                 chain.Auth        = (*TestAuth)(nil)
	_                 chain.AuthEngines = (*TestAuthEngines)(nil)
)

type TestAuth struct {
	NumComputeUnits uint64        `serialize:"true" json:"numComputeUnits"`
	ActorAddress    codec.Address `serialize:"true" json:"actor"`
	SponsorAddress  codec.Address `serialize:"true" json:"sponsor"`
	ShouldErr       bool          `serialize:"true" json:"shouldErr"`
	Start           int64         `serialize:"true" json:"start"`
	End             int64         `serialize:"true" json:"end"`
}

func NewDummyTestAuth() *TestAuth {
	return &TestAuth{
		NumComputeUnits: 1,
		ActorAddress:    codec.Address{1, 2, 3},
		SponsorAddress:  codec.Address{1, 2, 3},
		Start:           -1,
		End:             -1,
	}
}

func (*TestAuth) GetTypeID() uint8 {
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
	return t.Start, t.End
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

func (t *TestAuth) Verify(_ context.Context, _ []byte) error {
	if t.ShouldErr {
		return ErrTestAuthVerify
	}
	return nil
}

type TestAuthFactory struct {
	TestAuth *TestAuth
}

func (t *TestAuthFactory) Sign(_ []byte) (chain.Auth, error) {
	return t.TestAuth, nil
}

func (t *TestAuthFactory) MaxUnits() (bandwidth uint64, compute uint64) {
	return uint64(len(t.TestAuth.Bytes())), t.TestAuth.NumComputeUnits
}

func (t *TestAuthFactory) Address() codec.Address {
	return t.TestAuth.ActorAddress
}

type TestAuthEngines struct {
	GetAuthBatchVerifierF func(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool)
}

func (t *TestAuthEngines) GetAuthBatchVerifier(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool) {
	return t.GetAuthBatchVerifierF(authTypeID, cores, count)
}

// NewDummyTestAuthEngines returns an instance of TestAuthEngines with no-op implementations
func NewDummyTestAuthEngines() *TestAuthEngines {
	return &TestAuthEngines{
		GetAuthBatchVerifierF: func(uint8, int, int) (chain.AuthBatchVerifier, bool) {
			return nil, false
		},
	}
}

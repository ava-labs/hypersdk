// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"context"
	"fmt"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Auth = (*SECP256R1)(nil)

const (
	SECP256R1ComputeUnits = 10 // can't be batched like ed25519
	SECP256R1Size         = 1 + secp256r1.PublicKeyLen + secp256r1.SignatureLen
)

type SECP256R1 struct {
	Signer    secp256r1.PublicKey `json:"signer"`
	Signature secp256r1.Signature `json:"signature"`

	addr codec.Address
}

func (d *SECP256R1) address() codec.Address {
	if d.addr == codec.EmptyAddress {
		d.addr = NewSECP256R1Address(d.Signer)
	}
	return d.addr
}

func (*SECP256R1) GetTypeID() uint8 {
	return SECP256R1ID
}

func (*SECP256R1) ComputeUnits(chain.Rules) uint64 {
	return SECP256R1ComputeUnits
}

func (*SECP256R1) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

func (d *SECP256R1) Verify(_ context.Context, msg []byte) error {
	if !secp256r1.Verify(msg, d.Signer, d.Signature) {
		return crypto.ErrInvalidSignature
	}
	return nil
}

func (d *SECP256R1) Actor() codec.Address {
	return d.address()
}

func (d *SECP256R1) Sponsor() codec.Address {
	return d.address()
}

func (d *SECP256R1) Bytes() []byte {
	b := make([]byte, SECP256R1Size)
	b[0] = SECP256R1ID
	copy(b[1:], d.Signer[:])
	copy(b[1+secp256r1.PublicKeyLen:], d.Signature[:])
	return b
}

func UnmarshalSECP256R1(bytes []byte) (chain.Auth, error) {
	if len(bytes) != SECP256R1Size {
		return nil, fmt.Errorf("invalid secp256r1 auth size %d != %d", len(bytes), ED25519Size)
	}

	if bytes[0] != SECP256R1ID {
		return nil, fmt.Errorf("unexpected secp256r1 typeID: %d != %d", bytes[0], SECP256R1ID)
	}

	var d SECP256R1
	copy(d.Signer[:], bytes[1:])
	copy(d.Signature[:], bytes[1+secp256r1.PublicKeyLen:])
	return &d, nil
}

var _ chain.AuthFactory = (*SECP256R1Factory)(nil)

type SECP256R1Factory struct {
	priv secp256r1.PrivateKey
}

func NewSECP256R1Factory(priv secp256r1.PrivateKey) *SECP256R1Factory {
	return &SECP256R1Factory{priv}
}

func (d *SECP256R1Factory) Sign(msg []byte) (chain.Auth, error) {
	sig, err := secp256r1.Sign(msg, d.priv)
	if err != nil {
		return nil, err
	}
	return &SECP256R1{Signer: d.priv.PublicKey(), Signature: sig}, nil
}

func (*SECP256R1Factory) MaxUnits() (uint64, uint64) {
	return SECP256R1Size, SECP256R1ComputeUnits
}

func (d *SECP256R1Factory) Address() codec.Address {
	return NewSECP256R1Address(d.priv.PublicKey())
}

func NewSECP256R1Address(pk secp256r1.PublicKey) codec.Address {
	return codec.CreateAddress(SECP256R1ID, utils.ToID(pk[:]))
}

type SECP256R1PrivateKeyFactory struct{}

func NewSECP256R1PrivateKeyFactory() *SECP256R1PrivateKeyFactory {
	return &SECP256R1PrivateKeyFactory{}
}

func (*SECP256R1PrivateKeyFactory) GeneratePrivateKey() (*PrivateKey, error) {
	p, err := secp256r1.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	return &PrivateKey{
		Address: NewSECP256R1Address(p.PublicKey()),
		Bytes:   p[:],
	}, nil
}

func (*SECP256R1PrivateKeyFactory) LoadPrivateKey(p []byte) (*PrivateKey, error) {
	if len(p) != secp256r1.PrivateKeyLen {
		return nil, ErrInvalidPrivateKeySize
	}
	pk := secp256r1.PrivateKey(p)
	return &PrivateKey{
		Address: NewSECP256R1Address(pk.PublicKey()),
		Bytes:   p,
	}, nil
}

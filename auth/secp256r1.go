// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Auth[*SECP256R1] = (*SECP256R1)(nil)

const (
	SECP256R1ComputeUnits = 10 // can't be batched like ed25519
	SECP256R1Size         = secp256r1.PublicKeyLen + secp256r1.SignatureLen
)

type SECP256R1 struct {
	Signer    secp256r1.PublicKey `canoto:"fixed bytes,1" json:"signer"`
	Signature secp256r1.Signature `canoto:"fixed bytes,2" json:"signature"`

	addr codec.Address

	canotoData canotoData_SECP256R1
}

func (d *SECP256R1) address() codec.Address {
	if d.addr == codec.EmptyAddress {
		d.addr = NewSECP256R1Address(d.Signer)
	}
	return d.addr
}

func (*SECP256R1) ComputeUnits() uint64 {
	return SECP256R1ComputeUnits
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

var _ chain.AuthFactory[*SECP256R1] = (*SECP256R1Factory)(nil)

type SECP256R1Factory struct {
	priv secp256r1.PrivateKey `canoto:"fixed bytes,1"`

	canotoData canotoData_SECP256R1Factory
}

func NewSECP256R1Factory(priv secp256r1.PrivateKey) *SECP256R1Factory {
	return &SECP256R1Factory{priv: priv}
}

func (d *SECP256R1Factory) Sign(msg []byte) (*SECP256R1, error) {
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

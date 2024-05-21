// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Auth = (*SECP256R1)(nil)

const (
	SECP256R1ComputeUnits = 10 // can't be batched like ed25519
	SECP256R1Size         = secp256r1.PublicKeyLen + secp256r1.SignatureLen
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
	return consts.SECP256R1ID
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

func (*SECP256R1) Size() int {
	return SECP256R1Size
}

func (d *SECP256R1) Marshal(p *codec.Packer) {
	p.PackFixedBytes(d.Signer[:])
	p.PackFixedBytes(d.Signature[:])
}

func UnmarshalSECP256R1(p *codec.Packer) (chain.Auth, error) {
	var d SECP256R1
	signer := d.Signer[:] // avoid allocating additional memory
	p.UnpackFixedBytes(secp256r1.PublicKeyLen, &signer)
	signature := d.Signature[:] // avoid allocating additional memory
	p.UnpackFixedBytes(secp256r1.SignatureLen, &signature)
	return &d, p.Err()
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

func NewSECP256R1Address(pk secp256r1.PublicKey) codec.Address {
	return codec.CreateAddress(consts.SECP256R1ID, utils.ToID(pk[:]))
}

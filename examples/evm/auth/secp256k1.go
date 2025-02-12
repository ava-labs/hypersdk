// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/examples/evm/crypto/secp256k1"

	hauth "github.com/ava-labs/hypersdk/auth"
	hcrypto "github.com/ava-labs/hypersdk/crypto"
)

var _ chain.Auth = (*SECP256K1)(nil)

const (
	SECP256K1ID           uint8 = 3
	SECP256K1ComputeUnits       = 10
	SECP256K1Size               = secp256k1.PublicKeyLen + secp256k1.SignatureLen
)

type SECP256K1 struct {
	Signer    secp256k1.PublicKey `json:"signer"`
	Signature secp256k1.Signature `json:"signature"`

	addr codec.Address
}

func (d *SECP256K1) address() codec.Address {
	if d.addr == codec.EmptyAddress {
		d.addr = NewSECP256K1Address(d.Signer)
	}
	return d.addr
}

func (*SECP256K1) GetTypeID() uint8 {
	return SECP256K1ID
}

func (*SECP256K1) ComputeUnits(chain.Rules) uint64 {
	return SECP256K1ComputeUnits
}

func (*SECP256K1) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

func (d *SECP256K1) Verify(_ context.Context, msg []byte) error {
	if !secp256k1.Verify(msg, d.Signer, d.Signature) {
		return hcrypto.ErrInvalidSignature
	}
	return nil
}

func (d *SECP256K1) Actor() codec.Address {
	return d.address()
}

func (d *SECP256K1) Sponsor() codec.Address {
	return d.address()
}

func (*SECP256K1) Size() int {
	return SECP256K1Size
}

func (d *SECP256K1) Marshal(p *codec.Packer) {
	p.PackFixedBytes(d.Signer[:])
	p.PackFixedBytes(d.Signature[:])
}

func UnmarshalSECP256K1(p *codec.Packer) (chain.Auth, error) {
	var d SECP256K1
	signer := d.Signer[:] // avoid allocating additional memory
	p.UnpackFixedBytes(secp256k1.PublicKeyLen, &signer)
	signature := d.Signature[:] // avoid allocating additional memory
	p.UnpackFixedBytes(secp256k1.SignatureLen, &signature)
	return &d, p.Err()
}

var _ chain.AuthFactory = (*SECP256K1Factory)(nil)

type SECP256K1Factory struct {
	priv secp256k1.PrivateKey
}

func NewSECP256K1Factory(priv secp256k1.PrivateKey) *SECP256K1Factory {
	return &SECP256K1Factory{priv}
}

func (d *SECP256K1Factory) Sign(msg []byte) (chain.Auth, error) {
	sig := d.priv.Sign(msg)
	return &SECP256K1{Signer: d.priv.PublicKey(), Signature: sig}, nil
}

func (*SECP256K1Factory) MaxUnits() (uint64, uint64) {
	return SECP256K1Size, SECP256K1ComputeUnits
}

func (d *SECP256K1Factory) Address() codec.Address {
	return NewSECP256K1Address(d.priv.PublicKey())
}

func NewSECP256K1Address(pk secp256k1.PublicKey) codec.Address {
	// return codec.CreateAddress(SECP256K1ID, utils.ToID(pk[:]))
	addr, err := secp256k1.PublicKeyToAddress(pk)
	if err != nil {
		return codec.EmptyAddress
	}

	paddedAddr := codec.Address{}
	paddedAddr[0] = SECP256K1ID
	id := ids.ID{}
	copy(id[12:], addr[:])
	copy(paddedAddr[1:], id[:])
	fmt.Println("paddedAddr: ", paddedAddr)
	fmt.Println("addr: ", addr)
	return paddedAddr
}

type SECP256K1PrivateKeyFactory struct{}

func NewSECP256K1PrivateKeyFactory() *SECP256K1PrivateKeyFactory {
	return &SECP256K1PrivateKeyFactory{}
}

func (*SECP256K1PrivateKeyFactory) GeneratePrivateKey() (*hauth.PrivateKey, error) {
	p, err := secp256k1.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	return &hauth.PrivateKey{
		Address: NewSECP256K1Address(p.PublicKey()),
		Bytes:   p[:],
	}, nil
}

func (*SECP256K1PrivateKeyFactory) LoadPrivateKey(p []byte) (*hauth.PrivateKey, error) {
	if len(p) != secp256k1.PrivateKeyLen {
		return nil, hauth.ErrInvalidPrivateKeySize
	}
	pk := secp256k1.PrivateKey(p)
	return &hauth.PrivateKey{
		Address: NewSECP256K1Address(pk.PublicKey()),
		Bytes:   p,
	}, nil
}

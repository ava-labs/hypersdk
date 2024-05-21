// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Auth = (*BLS)(nil)

const (
	BLSComputeUnits = 10
	BLSSize         = bls.PublicKeyLen + bls.SignatureLen
)

type BLS struct {
	Signer    *bls.PublicKey `json:"signer,omitempty"`
	Signature *bls.Signature `json:"signature,omitempty"`

	addr codec.Address
}

func (b *BLS) address() codec.Address {
	if b.addr == codec.EmptyAddress {
		b.addr = NewBLSAddress(b.Signer)
	}
	return b.addr
}

func (*BLS) GetTypeID() uint8 {
	return consts.BLSID
}

func (*BLS) ComputeUnits(chain.Rules) uint64 {
	return BLSComputeUnits
}

func (*BLS) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

func (b *BLS) Verify(_ context.Context, msg []byte) error {
	if !bls.Verify(msg, b.Signer, b.Signature) {
		return crypto.ErrInvalidSignature
	}
	return nil
}

func (b *BLS) Actor() codec.Address {
	return b.address()
}

func (b *BLS) Sponsor() codec.Address {
	return b.address()
}

func (*BLS) Size() int {
	return BLSSize
}

func (b *BLS) Marshal(p *codec.Packer) {
	p.PackFixedBytes(bls.PublicKeyToBytes(b.Signer))
	p.PackFixedBytes(bls.SignatureToBytes(b.Signature))
}

func UnmarshalBLS(p *codec.Packer) (chain.Auth, error) {
	var b BLS

	signer := make([]byte, bls.PublicKeyLen)
	p.UnpackFixedBytes(bls.PublicKeyLen, &signer)
	signature := make([]byte, bls.SignatureLen)
	p.UnpackFixedBytes(bls.SignatureLen, &signature)

	pk, err := bls.PublicKeyFromBytes(signer)
	if err != nil {
		return nil, err
	}
	b.Signer = pk

	sig, err := bls.SignatureFromBytes(signature)
	if err != nil {
		return nil, err
	}

	b.Signature = sig
	return &b, p.Err()
}

var _ chain.AuthFactory = (*BLSFactory)(nil)

type BLSFactory struct {
	priv *bls.PrivateKey
}

func NewBLSFactory(priv *bls.PrivateKey) *BLSFactory {
	return &BLSFactory{priv}
}

func (b *BLSFactory) Sign(msg []byte) (chain.Auth, error) {
	return &BLS{Signer: bls.PublicFromPrivateKey(b.priv), Signature: bls.Sign(msg, b.priv)}, nil
}

func (*BLSFactory) MaxUnits() (uint64, uint64) {
	return BLSSize, BLSComputeUnits
}

func NewBLSAddress(pk *bls.PublicKey) codec.Address {
	return codec.CreateAddress(consts.BLSID, utils.ToID(bls.PublicKeyToBytes(pk)))
}

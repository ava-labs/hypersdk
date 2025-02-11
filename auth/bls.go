// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"context"

	"github.com/StephenButtolph/canoto"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Auth[*BLS] = (*BLS)(nil)

const (
	BLSComputeUnits = 10
	BLSSize         = bls.PublicKeyLen + bls.SignatureLen
)

type blsCanoto struct {
	PublicKeyBytes [bls.PublicKeyLen]byte `canoto:"fixed bytes,1" json:"signer"`
	Signaturebytes [bls.SignatureLen]byte `canoto:"fixed bytes,2" json:"signature"`

	canotoData canotoData_blsCanoto
}

type BLS struct {
	// embed blsCanoto so that we inherit all the canoto marshalling functions
	blsCanoto
	Signer    *bls.PublicKey
	Signature *bls.Signature

	addr codec.Address
}

func (b *BLS) MakeCanoto() *BLS {
	return new(BLS)
}

func (b *BLS) UnmarshalCanoto(bytes []byte) error {
	r := canoto.Reader{
		B: bytes,
	}
	return b.UnmarshalCanotoFrom(r)
}

func (b *BLS) UnmarshalCanotoFrom(r canoto.Reader) error {
	if err := b.blsCanoto.UnmarshalCanotoFrom(r); err != nil {
		return err
	}
	signer, err := bls.PublicKeyFromBytes(b.PublicKeyBytes[:])
	if err != nil {
		return err
	}
	signature, err := bls.SignatureFromBytes(b.Signaturebytes[:])
	if err != nil {
		return err
	}
	b.Signer = signer
	b.Signature = signature
	return nil
}

func (*BLS) ComputeUnits() uint64 {
	return BLSComputeUnits
}

func (b *BLS) Verify(_ context.Context, msg []byte) error {
	if !bls.Verify(msg, b.Signer, b.Signature) {
		return crypto.ErrInvalidSignature
	}
	return nil
}

func (b *BLS) address() codec.Address {
	if b.addr == codec.EmptyAddress {
		b.addr = NewBLSAddress(b.Signer)
	}
	return b.addr
}

func (b *BLS) Actor() codec.Address {
	return b.address()
}

func (b *BLS) Sponsor() codec.Address {
	return b.address()
}

var _ chain.AuthFactory[*BLS] = (*BLSFactory)(nil)

type blsFactoryCanoto struct {
	Priv [bls.PrivateKeyLen]byte `canoto:"fixed bytes,1"`

	canotoData canotoData_blsFactoryCanoto
}

type BLSFactory struct {
	blsFactoryCanoto
	priv *bls.PrivateKey
}

func NewBLSFactory(priv *bls.PrivateKey) *BLSFactory {
	return &BLSFactory{priv: priv}
}

func (b *BLSFactory) MakeCanoto() *BLSFactory {
	return new(BLSFactory)
}

func (b *BLSFactory) UnmarshalCanoto(bytes []byte) error {
	r := canoto.Reader{
		B: bytes,
	}
	return b.UnmarshalCanotoFrom(r)
}

func (b *BLSFactory) UnmarshalCanotoFrom(r canoto.Reader) error {
	if err := b.blsFactoryCanoto.UnmarshalCanotoFrom(r); err != nil {
		return err
	}
	privKey, err := bls.PrivateKeyFromBytes(b.Priv[:])
	if err != nil {
		return err
	}
	b.priv = privKey
	return nil
}

func (b *BLSFactory) Sign(msg []byte) (*BLS, error) {
	blsAuth := &BLS{
		Signer:    bls.PublicFromPrivateKey(b.priv),
		Signature: bls.Sign(msg, b.priv),
	}
	signer, err := bls.PublicKeyFromBytes(blsAuth.PublicKeyBytes[:])
	if err != nil {
		return nil, err
	}
	signature, err := bls.SignatureFromBytes(blsAuth.Signaturebytes[:])
	if err != nil {
		return nil, err
	}
	blsAuth.Signer = signer
	blsAuth.Signature = signature
	return blsAuth, nil
}

func (*BLSFactory) MaxUnits() (uint64, uint64) {
	return BLSSize, BLSComputeUnits
}

func (b *BLSFactory) Address() codec.Address {
	return NewBLSAddress(bls.PublicFromPrivateKey(b.priv))
}

func NewBLSAddress(pk *bls.PublicKey) codec.Address {
	return codec.CreateAddress(BLSID, utils.ToID(bls.PublicKeyToBytes(pk)))
}

type BLSPrivateKeyFactory struct{}

func NewBLSPrivateKeyFactory() *BLSPrivateKeyFactory {
	return &BLSPrivateKeyFactory{}
}

func (*BLSPrivateKeyFactory) GeneratePrivateKey() (*PrivateKey, error) {
	p, err := bls.GeneratePrivateKey()
	if err != nil {
		return nil, err
	}
	return &PrivateKey{
		Address: NewBLSAddress(bls.PublicFromPrivateKey(p)),
		Bytes:   bls.PrivateKeyToBytes(p),
	}, nil
}

func (*BLSPrivateKeyFactory) LoadPrivateKey(privateKey []byte) (*PrivateKey, error) {
	if len(privateKey) != bls.PrivateKeyLen {
		return nil, ErrInvalidPrivateKeySize
	}
	privKey, err := bls.PrivateKeyFromBytes(privateKey)
	if err != nil {
		return nil, err
	}
	return &PrivateKey{
		Address: NewBLSAddress(bls.PublicFromPrivateKey(privKey)),
		Bytes:   privateKey,
	}, nil
}

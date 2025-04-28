// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

//go:generate go run github.com/StephenButtolph/canoto/canoto $GOFILE

import (
	"context"
	"fmt"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Auth = (*BLS)(nil)

const (
	BLSComputeUnits = 10
	BLSSize         = 1 + bls.PublicKeyLen + bls.SignatureLen
)

type BLS struct {
	SignerBytes    []byte `canoto:"bytes,1" json:"signerBytes,omitempty"`
	SignatureBytes []byte `canoto:"bytes,2" json:"signatureBytes,omitempty"`

	Signer    *bls.PublicKey `json:"signer,omitempty"`
	Signature *bls.Signature `json:"signature,omitempty"`

	addr codec.Address

	canotoData canotoData_BLS
}

func (b *BLS) address() codec.Address {
	if b.addr == codec.EmptyAddress {
		b.addr = NewBLSAddress(b.Signer)
	}
	return b.addr
}

func (*BLS) GetTypeID() uint8 {
	return BLSID
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

func (b *BLS) Bytes() []byte {
	return append([]byte{BLSID}, b.MarshalCanoto()...)
}

func UnmarshalBLS(bytes []byte) (chain.Auth, error) {
	if bytes[0] != BLSID {
		return nil, fmt.Errorf("unexpected BLS typeID: %d != %d", bytes[0], BLSID)
	}

	b := &BLS{}
	if err := b.UnmarshalCanoto(bytes[1:]); err != nil {
		return nil, err
	}

	// populate pointer fields
	publicKey, err := bls.PublicKeyFromBytes(b.SignerBytes)
	if err != nil {
		return nil, err
	}

	signature, err := bls.SignatureFromBytes(b.SignatureBytes)
	if err != nil {
		return nil, err
	}

	b.Signer = publicKey
	b.Signature = signature

	return b, nil
}

var _ chain.AuthFactory = (*BLSFactory)(nil)

type BLSFactory struct {
	priv *bls.PrivateKey
}

func NewBLSFactory(priv *bls.PrivateKey) *BLSFactory {
	return &BLSFactory{priv}
}

func (b *BLSFactory) Sign(msg []byte) (chain.Auth, error) {
	signature, err := bls.Sign(msg, b.priv)
	if err != nil {
		return nil, err
	}

	signer := bls.PublicFromPrivateKey(b.priv)

	return &BLS{
		SignerBytes:    bls.PublicKeyToBytes(signer),
		SignatureBytes: bls.SignatureToBytes(signature),
		Signer:         signer,
		Signature:      signature,
	}, nil
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

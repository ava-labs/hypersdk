// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"context"

	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
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

func (*BLS) MaxComputeUnits(chain.Rules) uint64 {
	return BLSComputeUnits
}

func (*BLS) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

func (b *BLS) StateKeys() []string {
	return []string{
		string(storage.BalanceKey(b.address())),
	}
}

func (b *BLS) AsyncVerify(msg []byte) error {
	if !bls.Verify(msg, b.Signer, b.Signature) {
		return crypto.ErrInvalidSignature
	}
	return nil
}

func (b *BLS) Verify(
	_ context.Context,
	r chain.Rules,
	_ state.Immutable,
	_ chain.Action,
) (uint64, error) {
	// We don't do anything during verify (there is no additional state to check
	// to authorize the signer other than verifying the signature)
	return b.MaxComputeUnits(r), nil
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

func UnmarshalBLS(p *codec.Packer, _ *warp.Message) (chain.Auth, error) {
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

func (b *BLS) CanDeduct(
	ctx context.Context,
	im state.Immutable,
	amount uint64,
) error {
	bal, err := storage.GetBalance(ctx, im, b.address())
	if err != nil {
		return err
	}
	if bal < amount {
		return storage.ErrInvalidBalance
	}
	return nil
}

func (b *BLS) Deduct(
	ctx context.Context,
	mu state.Mutable,
	amount uint64,
) error {
	return storage.SubBalance(ctx, mu, b.address(), amount)
}

func (b *BLS) Refund(
	ctx context.Context,
	mu state.Mutable,
	amount uint64,
) error {
	// Don't create account if it doesn't exist (may have sent all funds).
	return storage.AddBalance(ctx, mu, b.address(), amount, false)
}

var _ chain.AuthFactory = (*BLSFactory)(nil)

type BLSFactory struct {
	priv *bls.PrivateKey
}

func NewBLSFactory(priv *bls.PrivateKey) *BLSFactory {
	return &BLSFactory{priv}
}

func (b *BLSFactory) Sign(msg []byte, _ chain.Action) (chain.Auth, error) {
	return &BLS{Signer: bls.PublicFromPrivateKey(b.priv), Signature: bls.Sign(msg, b.priv)}, nil
}

func (*BLSFactory) MaxUnits() (uint64, uint64, []uint16) {
	return BLSSize, BLSComputeUnits, []uint16{storage.BalanceChunks}
}

func NewBLSAddress(pk *bls.PublicKey) codec.Address {
	return codec.CreateAddress(consts.BLSID, utils.ToID(bls.PublicKeyToBytes(pk)))
}

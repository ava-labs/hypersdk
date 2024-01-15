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
	BLSCompressSize	= bls.PublicKeyLen * 2	
	BLSSize         = BLSCompressSize + bls.SignatureLen
)

type BLS struct {
	Signer    *bls.PublicKey `json:"signer,omitempty"`
	Signature *bls.Signature `json:"signature,omitempty"`

	addr codec.Address
}

func (d *BLS) address() codec.Address {
	if d.addr == codec.EmptyAddress {
		d.addr = NewBLSAddress(d.Signer)
	}
	return d.addr
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

func (d *BLS) StateKeys() []string {
	return []string{
		string(storage.BalanceKey(d.address())),
	}
}

func (d *BLS) AsyncVerify(msg []byte) error {
	if !bls.Verify(msg, d.Signer, d.Signature) {
		return crypto.ErrInvalidSignature
	}
	return nil
}

func (d *BLS) Verify(
	_ context.Context,
	r chain.Rules,
	_ state.Immutable,
	_ chain.Action,
) (uint64, error) {
	// We don't do anything during verify (there is no additional state to check
	// to authorize the signer other than verifying the signature)
	return d.MaxComputeUnits(r), nil
}

func (d *BLS) Actor() codec.Address {
	return d.address()
}

func (d *BLS) Sponsor() codec.Address {
	return d.address()
}

func (*BLS) Size() int {
	return BLSSize
}

func (d *BLS) Marshal(p *codec.Packer) {
	p.PackFixedBytes(bls.SerializePublicKey(d.Signer))
	p.PackFixedBytes(bls.SignatureToBytes(d.Signature))
}

func UnmarshalBLS(p *codec.Packer, _ *warp.Message) (chain.Auth, error) {
	var d BLS

	signer := make([]byte, BLSCompressSize)
	// Public key length is defined to be 48 bytes.
	// When we serialize, this results in a length twice of that
	// https://github.com/supranational/blst/blob/master/bindings/go/blst.go#L165C41-L165C41
	p.UnpackFixedBytes(BLSCompressSize, &signer)
	signature := make([]byte, bls.SignatureLen)
	p.UnpackFixedBytes(bls.SignatureLen, &signature)

	d.Signer = bls.DeserializePublicKey(signer)
	sig, err := bls.SignatureFromBytes(signature)
	if err != nil {
		return &BLS{}, nil
	}

	d.Signature = sig
	return &d, p.Err()
}

func (d *BLS) CanDeduct(
	ctx context.Context,
	im state.Immutable,
	amount uint64,
) error {
	bal, err := storage.GetBalance(ctx, im, d.address())
	if err != nil {
		return err
	}
	if bal < amount {
		return storage.ErrInvalidBalance
	}
	return nil
}

func (d *BLS) Deduct(
	ctx context.Context,
	mu state.Mutable,
	amount uint64,
) error {
	return storage.SubBalance(ctx, mu, d.address(), amount)
}

func (d *BLS) Refund(
	ctx context.Context,
	mu state.Mutable,
	amount uint64,
) error {
	// Don't create account if it doesn't exist (may have sent all funds).
	return storage.AddBalance(ctx, mu, d.address(), amount, false)
}

var _ chain.AuthFactory = (*BLSFactory)(nil)

type BLSFactory struct {
	priv *bls.PrivateKey `json:"priv,omitempty"`
}

func NewBLSFactory(priv *bls.PrivateKey) *BLSFactory {
	return &BLSFactory{priv}
}

func (d *BLSFactory) Sign(msg []byte, _ chain.Action) (chain.Auth, error) {
	sig := bls.Sign(msg, d.priv)
	return &BLS{Signer: bls.PublicFromPrivateKey(d.priv), Signature: sig}, nil
}

func (*BLSFactory) MaxUnits() (uint64, uint64, []uint16) {
	return BLSSize, BLSComputeUnits, []uint16{storage.BalanceChunks}
}

func NewBLSAddress(pk *bls.PublicKey) codec.Address {
	return codec.CreateAddress(consts.BLSID, utils.ToID(bls.SerializePublicKey(pk)))
}

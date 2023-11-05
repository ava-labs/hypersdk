// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"context"

	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
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

func (*SECP256R1) MaxComputeUnits(chain.Rules) uint64 {
	return SECP256R1ComputeUnits
}

func (*SECP256R1) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

func (d *SECP256R1) StateKeys() []string {
	return []string{
		string(storage.BalanceKey(d.address())),
	}
}

func (d *SECP256R1) AsyncVerify(msg []byte) error {
	if !secp256r1.Verify(msg, d.Signer, d.Signature) {
		return crypto.ErrInvalidSignature
	}
	return nil
}

func (d *SECP256R1) Verify(
	_ context.Context,
	r chain.Rules,
	_ state.Immutable,
	_ chain.Action,
) (uint64, error) {
	// We don't do anything during verify (there is no additional state to check
	// to authorize the signer other than verifying the signature)
	return d.MaxComputeUnits(r), nil
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

func UnmarshalSECP256R1(p *codec.Packer, _ *warp.Message) (chain.Auth, error) {
	var d SECP256R1
	signer := d.Signer[:] // avoid allocating additional memory
	p.UnpackFixedBytes(secp256r1.PublicKeyLen, &signer)
	signature := d.Signature[:] // avoid allocating additional memory
	p.UnpackFixedBytes(secp256r1.SignatureLen, &signature)
	return &d, p.Err()
}

func (d *SECP256R1) CanDeduct(
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

func (d *SECP256R1) Deduct(
	ctx context.Context,
	mu state.Mutable,
	amount uint64,
) error {
	return storage.SubBalance(ctx, mu, d.address(), amount)
}

func (d *SECP256R1) Refund(
	ctx context.Context,
	mu state.Mutable,
	amount uint64,
) error {
	// Don't create account if it doesn't exist (may have sent all funds).
	return storage.AddBalance(ctx, mu, d.address(), amount, false)
}

var _ chain.AuthFactory = (*SECP256R1Factory)(nil)

type SECP256R1Factory struct {
	priv secp256r1.PrivateKey
}

func NewSECP256R1Factory(priv secp256r1.PrivateKey) *SECP256R1Factory {
	return &SECP256R1Factory{priv}
}

func (d *SECP256R1Factory) Sign(msg []byte, _ chain.Action) (chain.Auth, error) {
	sig, err := secp256r1.Sign(msg, d.priv)
	if err != nil {
		return nil, err
	}
	return &SECP256R1{Signer: d.priv.PublicKey(), Signature: sig}, nil
}

func (*SECP256R1Factory) MaxUnits() (uint64, uint64, []uint16) {
	return SECP256R1Size, SECP256R1ComputeUnits, []uint16{storage.BalanceChunks}
}

func NewSECP256R1Address(pk secp256r1.PublicKey) codec.Address {
	return codec.CreateAddress(consts.SECP256R1ID, utils.ToID(pk[:]))
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
)

var _ chain.Auth = (*ED25519)(nil)

type ED25519 struct {
	Signer    crypto.PublicKey `json:"signer"`
	Signature crypto.Signature `json:"signature"`
}

func (*ED25519) MaxUnits(
	chain.Rules,
) uint64 {
	return crypto.PublicKeyLen + crypto.SignatureLen*5 // make signatures more expensive
}

func (*ED25519) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

func (d *ED25519) StateKeys() [][]byte {
	return [][]byte{
		// We always pay fees with the native asset (which is [ids.Empty])
		storage.PrefixBalanceKey(d.Signer, ids.Empty),
	}
}

func (d *ED25519) AsyncVerify(msg []byte) error {
	if !crypto.Verify(msg, d.Signer, d.Signature) {
		return ErrInvalidSignature
	}
	return nil
}

func (d *ED25519) Verify(
	_ context.Context,
	r chain.Rules,
	_ chain.Database,
	_ chain.Action,
) (uint64, error) {
	// We don't do anything during verify (there is no additional state to check
	// to authorize the signer other than verifying the signature)
	return d.MaxUnits(r), nil
}

func (d *ED25519) Payer() []byte {
	return d.Signer[:]
}

func (d *ED25519) Marshal(p *codec.Packer) {
	p.PackPublicKey(d.Signer)
	p.PackSignature(d.Signature)
}

func UnmarshalED25519(p *codec.Packer, _ *warp.Message) (chain.Auth, error) {
	var d ED25519
	p.UnpackPublicKey(true, &d.Signer)
	p.UnpackSignature(&d.Signature)
	return &d, p.Err()
}

func (d *ED25519) CanDeduct(
	ctx context.Context,
	db chain.Database,
	amount uint64,
) error {
	bal, err := storage.GetBalance(ctx, db, d.Signer, ids.Empty)
	if err != nil {
		return err
	}
	if bal < amount {
		return storage.ErrInvalidBalance
	}
	return nil
}

func (d *ED25519) Deduct(
	ctx context.Context,
	db chain.Database,
	amount uint64,
) error {
	return storage.SubBalance(ctx, db, d.Signer, ids.Empty, amount)
}

func (d *ED25519) Refund(
	ctx context.Context,
	db chain.Database,
	amount uint64,
) error {
	return storage.AddBalance(ctx, db, d.Signer, ids.Empty, amount)
}

var _ chain.AuthFactory = (*ED25519Factory)(nil)

func NewED25519Factory(priv crypto.PrivateKey) *ED25519Factory {
	return &ED25519Factory{priv}
}

type ED25519Factory struct {
	priv crypto.PrivateKey
}

func (d *ED25519Factory) Sign(msg []byte, _ chain.Action) (chain.Auth, error) {
	sig := crypto.Sign(msg, d.priv)
	return &ED25519{d.priv.PublicKey(), sig}, nil
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/storage"
)

var _ chain.Auth = (*Direct)(nil)

type Direct struct {
	Signer    crypto.PublicKey `json:"signer"`
	Signature crypto.Signature `json:"signature"`
}

func (*Direct) MaxUnits(
	chain.Rules,
) uint64 {
	return crypto.PublicKeyLen + crypto.SignatureLen*5 // make signatures more expensive
}

func (*Direct) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

func (d *Direct) StateKeys() [][]byte {
	return [][]byte{
		storage.PrefixBalanceKey(d.Signer), // fee payer
		storage.PrefixPermissionsKey(d.Signer, d.Signer),
	}
}

func (d *Direct) AsyncVerify(msg []byte) error {
	if !crypto.Verify(msg, d.Signer, d.Signature) {
		return ErrInvalidSignature
	}
	return nil
}

// Verify could be used to perform complex ACL rules that require state access
// to check.
func (d *Direct) Verify(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	action chain.Action,
) (uint64, error) {
	// Could have modified perms before doing a simple signature so we must check
	// to make sure we are still authorized to act on behalf of [Signer]
	if err := Authorized(ctx, db, action, d.Signer, d.Signer, true); err != nil {
		return 0, err
	}
	return d.MaxUnits(r), nil
}

func (d *Direct) Payer() []byte {
	return d.Signer[:]
}

func (d *Direct) Marshal(p *codec.Packer) {
	p.PackPublicKey(d.Signer)
	p.PackSignature(d.Signature)
}

func UnmarshalDirect(p *codec.Packer) (chain.Auth, error) {
	var d Direct
	p.UnpackPublicKey(&d.Signer)
	p.UnpackSignature(&d.Signature)
	return &d, p.Err()
}

func (d *Direct) CanDeduct(
	ctx context.Context,
	db chain.Database,
	amount uint64,
) error {
	// Account must exist if [u] > 0
	u, _, err := storage.GetBalance(ctx, db, d.Signer)
	if err != nil {
		return err
	}
	if u < amount {
		return storage.ErrInvalidBalance
	}
	return nil
}

func (d *Direct) Deduct(
	ctx context.Context,
	db chain.Database,
	amount uint64,
) error {
	return storage.SubUnlockedBalance(ctx, db, d.Signer, amount)
}

func (d *Direct) Refund(
	ctx context.Context,
	db chain.Database,
	amount uint64,
) error {
	// Discard any funds returned if the account doesn't exist
	//
	// This could occur if the transaction closes out the account
	_, err := storage.AddUnlockedBalance(ctx, db, d.Signer, amount, true)
	return err
}

var _ chain.AuthFactory = (*DirectFactory)(nil)

func NewDirectFactory(priv crypto.PrivateKey) *DirectFactory {
	return &DirectFactory{priv}
}

type DirectFactory struct {
	priv crypto.PrivateKey
}

func (d *DirectFactory) Sign(msg []byte, _ chain.Action) (chain.Auth, error) {
	sig := crypto.Sign(msg, d.priv)
	return &Direct{d.priv.PublicKey(), sig}, nil
}

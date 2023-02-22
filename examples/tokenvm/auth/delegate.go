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

var _ chain.Auth = (*Delegate)(nil)

const (
	actorPaysBit uint8 = 0
)

type Delegate struct {
	Actor     crypto.PublicKey `json:"actor"`
	Signer    crypto.PublicKey `json:"signer"`
	Signature crypto.Signature `json:"signature"`

	ActorPays bool `json:"actorPays"`
}

func (d *Delegate) payer() crypto.PublicKey {
	if d.ActorPays {
		return d.Actor
	}
	return d.Signer
}

func (*Delegate) MaxUnits(
	chain.Rules,
) uint64 {
	return 1 + crypto.PublicKeyLen*2 + crypto.SignatureLen*5 // make signatures more expensive
}

func (*Delegate) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

func (d *Delegate) StateKeys() [][]byte {
	return [][]byte{
		storage.PrefixBalanceKey(d.payer()), // fee payer
		storage.PrefixPermissionsKey(d.Actor, d.Signer),
	}
}

func (d *Delegate) AsyncVerify(msg []byte) error {
	if !crypto.Verify(msg, d.Signer, d.Signature) {
		return ErrInvalidSignature
	}
	return nil
}

// Verify could be used to perform complex ACL rules that require state access
// to check.
func (d *Delegate) Verify(
	ctx context.Context,
	r chain.Rules,
	db chain.Database,
	action chain.Action,
) (uint64, error) {
	if d.Actor == d.Signer {
		return 0, ErrActorEqualsSigner
	}
	// Allowed will check the existence of actor
	//
	// Note: actor and signer equivalence does not mean an action is allowed (key
	// could have been rotated)
	if err := Authorized(ctx, db, action, d.Actor, d.Signer, d.ActorPays); err != nil {
		return 0, err
	}
	return d.MaxUnits(r), nil
}

func (d *Delegate) Payer() []byte {
	payer := d.payer()
	return payer[:]
}

func (d *Delegate) Marshal(p *codec.Packer) {
	p.PackPublicKey(d.Actor)
	p.PackPublicKey(d.Signer)
	p.PackSignature(d.Signature)
	p.PackBool(d.ActorPays)
}

func UnmarshalDelegate(p *codec.Packer) (chain.Auth, error) {
	var d Delegate
	p.UnpackPublicKey(&d.Actor)
	p.UnpackPublicKey(&d.Signer)
	p.UnpackSignature(&d.Signature)
	d.ActorPays = p.UnpackBool()
	return &d, p.Err()
}

func (d *Delegate) CanDeduct(
	ctx context.Context,
	db chain.Database,
	amount uint64,
) error {
	// Account must exist if [u] > 0
	u, _, err := storage.GetBalance(ctx, db, d.payer())
	if err != nil {
		return err
	}
	if u < amount {
		return storage.ErrInvalidBalance
	}
	return nil
}

func (d *Delegate) Deduct(
	ctx context.Context,
	db chain.Database,
	amount uint64,
) error {
	return storage.SubUnlockedBalance(ctx, db, d.payer(), amount)
}

func (d *Delegate) Refund(
	ctx context.Context,
	db chain.Database,
	amount uint64,
) error {
	// Discard any funds returned if the account doesn't exist
	//
	// This could occur if the transaction closes out the account
	_, err := storage.AddUnlockedBalance(ctx, db, d.payer(), amount, true)
	return err
}

var _ chain.AuthFactory = (*DelegateFactory)(nil)

func NewDelegateFactory(
	actor crypto.PublicKey,
	priv crypto.PrivateKey,
	actorPays bool,
) *DelegateFactory {
	return &DelegateFactory{actor, priv, actorPays}
}

type DelegateFactory struct {
	actor     crypto.PublicKey
	priv      crypto.PrivateKey
	actorPays bool
}

func (d *DelegateFactory) Sign(msg []byte, _ chain.Action) (chain.Auth, error) {
	sig := crypto.Sign(msg, d.priv)
	return &Delegate{d.actor, d.priv.PublicKey(), sig, d.actorPays}, nil
}

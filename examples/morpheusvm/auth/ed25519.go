// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"context"

	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
)

var _ chain.Auth = (*ED25519)(nil)

type ED25519 struct {
	Signer    ed25519.PublicKey `json:"signer"`
	Signature ed25519.Signature `json:"signature"`
}

func (*ED25519) GetTypeID() uint8 {
	return ed25519ID
}

func (*ED25519) MaxUnits(
	chain.Rules,
) uint64 {
	return ed25519.PublicKeyLen + ed25519.SignatureLen*5 // make signatures more expensive
}

func (*ED25519) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

func (d *ED25519) StateKeys() [][]byte {
	return [][]byte{
		// We always pay fees with the native asset (which is [ids.Empty])
		storage.PrefixBalanceKey(d.Signer),
	}
}

func (d *ED25519) AsyncVerify(msg []byte) error {
	if !ed25519.Verify(msg, d.Signer, d.Signature) {
		return ed25519.ErrInvalidSignature
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

func (*ED25519) Size() int {
	return ed25519.PublicKeyLen + ed25519.SignatureLen
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
	bal, err := storage.GetBalance(ctx, db, d.Signer)
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
	return storage.SubBalance(ctx, db, d.Signer, amount)
}

func (d *ED25519) Refund(
	ctx context.Context,
	db chain.Database,
	amount uint64,
) error {
	return storage.AddBalance(ctx, db, d.Signer, amount)
}

var _ chain.AuthFactory = (*ED25519Factory)(nil)

func NewED25519Factory(priv ed25519.PrivateKey) *ED25519Factory {
	return &ED25519Factory{priv}
}

type ED25519Factory struct {
	priv ed25519.PrivateKey
}

func (d *ED25519Factory) Sign(msg []byte, _ chain.Action) (chain.Auth, error) {
	sig := ed25519.Sign(msg, d.priv)
	return &ED25519{d.priv.PublicKey(), sig}, nil
}

type ED25519AuthEngine struct{}

func (*ED25519AuthEngine) GetBatchVerifier(cores int, count int) chain.AuthBatchVerifier {
	batchSize := math.Max(count/cores, 16)
	return &ED25519Batch{
		batchSize: batchSize,
		total:     count,
	}
}

func (*ED25519AuthEngine) Cache(auth chain.Auth) {
	pk := GetSigner(auth)
	ed25519.CachePublicKey(pk)
}

type ED25519Batch struct {
	batchSize int
	total     int

	counter      int
	totalCounter int
	batch        *ed25519.Batch
}

func (b *ED25519Batch) Add(msg []byte, rauth chain.Auth) func() error {
	auth := rauth.(*ED25519)
	if b.batch == nil {
		b.batch = ed25519.NewBatch(b.batchSize)
	}
	b.batch.Add(msg, auth.Signer, auth.Signature)
	b.counter++
	b.totalCounter++
	if b.counter == b.batchSize {
		last := b.batch
		b.counter = 0
		if b.totalCounter < b.total {
			// don't create a new batch if we are done
			b.batch = ed25519.NewBatch(b.batchSize)
		}
		return last.VerifyAsync()
	}
	return nil
}

func (b *ED25519Batch) Done() []func() error {
	if b.batch == nil {
		return nil
	}
	return []func() error{b.batch.VerifyAsync()}
}

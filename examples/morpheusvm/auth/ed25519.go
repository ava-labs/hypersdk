// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import (
	"context"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/utils"
)

var _ chain.Auth = (*ED25519)(nil)

const (
	ED25519ComputeUnits = 5
	ED25519Size         = ed25519.PublicKeyLen + ed25519.SignatureLen
)

type ED25519 struct {
	Signer    ed25519.PublicKey `json:"signer"`
	Signature ed25519.Signature `json:"signature"`

	addr codec.Address
}

func (d *ED25519) address() codec.Address {
	if d.addr == codec.EmptyAddress {
		d.addr = NewED25519Address(d.Signer)
	}
	return d.addr
}

func (*ED25519) GetTypeID() uint8 {
	return ed25519ID
}

func (*ED25519) ComputeUnits(chain.Rules) uint64 {
	return ED25519ComputeUnits
}

func (*ED25519) ValidRange(chain.Rules) (int64, int64) {
	return -1, -1
}

func (d *ED25519) Verify(_ context.Context, msg []byte) error {
	if !ed25519.Verify(msg, d.Signer, d.Signature) {
		return crypto.ErrInvalidSignature
	}
	return nil
}

func (d *ED25519) Actor() codec.Address {
	return d.address()
}

func (d *ED25519) Sponsor() codec.Address {
	return d.address()
}

func (*ED25519) Size() int {
	return ED25519Size
}

func (d *ED25519) Marshal(p *codec.Packer) {
	p.PackFixedBytes(d.Signer[:])
	p.PackFixedBytes(d.Signature[:])
}

func UnmarshalED25519(p *codec.Packer) (chain.Auth, error) {
	var d ED25519
	signer := d.Signer[:] // avoid allocating additional memory
	p.UnpackFixedBytes(ed25519.PublicKeyLen, &signer)
	signature := d.Signature[:] // avoid allocating additional memory
	p.UnpackFixedBytes(ed25519.SignatureLen, &signature)
	return &d, p.Err()
}

var _ chain.AuthFactory = (*ED25519Factory)(nil)

func NewED25519Factory(priv ed25519.PrivateKey) *ED25519Factory {
	return &ED25519Factory{priv}
}

type ED25519Factory struct {
	priv ed25519.PrivateKey
}

func (d *ED25519Factory) Sign(msg []byte) (chain.Auth, error) {
	sig := ed25519.Sign(msg, d.priv)
	return &ED25519{Signer: d.priv.PublicKey(), Signature: sig}, nil
}

func (*ED25519Factory) MaxUnits() (uint64, uint64) {
	return ED25519Size, ED25519ComputeUnits
}

type ED25519AuthEngine struct{}

func (*ED25519AuthEngine) GetBatchVerifier(cores int, count int) chain.AuthBatchVerifier {
	batchSize := max(count/cores, ed25519.MinBatchSize)
	return &ED25519Batch{
		batchSize: batchSize,
		total:     count,
	}
}

func (*ED25519AuthEngine) Cache(chain.Auth) {
	// TODO: add support for caching expanded public key to make batch verification faster
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

func NewED25519Address(pk ed25519.PublicKey) codec.Address {
	return codec.CreateAddress(ed25519ID, utils.ToID(pk[:]))
}

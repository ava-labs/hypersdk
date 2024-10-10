package dsmr

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
)

const ChunkCertificateSize = 136

type ChunkSignatureShare struct{}

func (ChunkSignatureShare) Verify(chunkID ids.ID) error { return nil }

func AggregateSignatureShares([]ChunkSignatureShare) ChunkSignature { return ChunkSignature{} }

type ChunkSignature struct{}

// TODO: add validator set context for verification
func (c ChunkSignature) Verify() error { return nil }

type ChunkCertificate struct {
	ChunkID ids.ID `serialize:"true"`
	Expiry  int64  `serialize:"true"`

	Signature ChunkSignature `serialize:"true"`
}

func (c *ChunkCertificate) Verify(_ context.Context) error {
	// TODO: verify slot
	return c.Signature.Verify()
}

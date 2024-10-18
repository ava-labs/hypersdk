// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	avautils "github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
)

const InitialChunkSize = 250 * 1024

// TODO emit configurable amount of chunks/sec
// TODO unexport
type Chunk[T Tx] struct {
	Producer    ids.NodeID             `serialize:"true"`
	Expiry      int64                  `serialize:"true"`
	Beneficiary codec.Address          `serialize:"true"`
	Txs         []T                    `serialize:"true"`
	Signer      [bls.PublicKeyLen]byte `serialize:"true"`
	Signature   [bls.SignatureLen]byte `serialize:"true"`

	bytes []byte
	id    ids.ID
}

func newChunk[T Tx](
	producer ids.NodeID,
	txs []T,
	expiry time.Time,
	beneficiary codec.Address,
	signer [bls.PublicKeyLen]byte,
	signature [bls.SignatureLen]byte,
) (Chunk[T], error) {
	c := Chunk[T]{
		Producer:    producer,
		Expiry:      expiry.Unix(),
		Beneficiary: beneficiary,
		Txs:         txs,
		Signer:      signer,
		Signature:   signature,
	}

	packer := wrappers.Packer{Bytes: make([]byte, 0, InitialChunkSize), MaxSize: consts.NetworkSizeLimit}
	if err := codec.LinearCodec.MarshalInto(c, &packer); err != nil {
		return Chunk[T]{}, err
	}
	c.bytes = packer.Bytes
	c.id = utils.ToID(c.bytes)
	return c, nil
}

func ParseChunk[T Tx](chunkBytes []byte) (Chunk[T], error) {
	c := Chunk[T]{}
	if err := codec.LinearCodec.UnmarshalFrom(&wrappers.Packer{Bytes: chunkBytes}, &c); err != nil {
		return Chunk[T]{}, err
	}
	c.bytes = chunkBytes
	c.id = utils.ToID(c.bytes)
	return c, nil
}

func parseChunkProto[T Tx](chunkProto *dsmr.Chunk) (Chunk[T], error) {
	producer, err := ids.ToNodeID(chunkProto.Producer)
	if err != nil {
		return Chunk[T]{}, err
	}

	beneficiary, err := codec.ToAddress(chunkProto.Beneficiary)
	if err != nil {
		return Chunk[T]{}, err
	}

	txs := make([]T, 0, len(chunkProto.Transactions))
	for _, tx := range chunkProto.Transactions {
		parsed := avautils.Zero[T]()
		packer := &wrappers.Packer{Bytes: tx.Bytes, MaxSize: consts.NetworkSizeLimit}
		if err := codec.LinearCodec.UnmarshalFrom(packer, &parsed); err != nil {
			return Chunk[T]{}, err
		}

		txs = append(txs, parsed)
	}

	signer := [bls.PublicKeyLen]byte{}
	signature := [bls.SignatureLen]byte{}
	copy(signer[:], chunkProto.Signer)
	copy(signature[:], chunkProto.Signature)

	return newChunk[T](
		producer,
		txs,
		time.Unix(chunkProto.Expiry, 0),
		beneficiary,
		signer,
		signature,
	)
}

// TODO implement
type ChunkFault struct{}

type Block struct {
	ID        ids.ID `serialize:"true"`
	ParentID  ids.ID `serialize:"true"`
	Height    uint64 `serialize:"true"`
	Timestamp int64  `serialize:"true"`

	// TODO Add TxsRoot?
	// TODO Add OutgoingWarpRoot?
	// TODO Do we need separate Available + Executed Chunks?
	Chunks []*ChunkCertificate `serialize:"true"`
}

// TODO where does this belong?
type ExecutionBlock[VerificationContext any] struct {
	Block

	vm VM[VerificationContext]
}

func (e *ExecutionBlock[V]) Verify(ctx context.Context, verificationContext V) error {
	for _, chunkCertificate := range e.Chunks {
		if err := chunkCertificate.Verify(ctx, verificationContext); err != nil {
			return err
		}
	}

	return e.vm.Verify(ctx, e)
}

func (e *ExecutionBlock[V]) Accept(ctx context.Context) error {
	return e.vm.Accept(ctx, e)
}

func (e *ExecutionBlock[V]) Reject(ctx context.Context) error {
	return e.vm.Reject(ctx, e)
}

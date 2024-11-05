// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
)

const InitialChunkSize = 250 * 1024

type Tx interface {
	GetID() ids.ID
	GetExpiry() int64
}

type UnsignedChunk[T Tx] struct {
	Producer    ids.NodeID    `serialize:"true"`
	Beneficiary codec.Address `serialize:"true"`
	Expiry      int64         `serialize:"true"`
	Txs         []T           `serialize:"true"`
}

// TODO emit configurable amount of chunks/sec
type Chunk[T Tx] struct {
	UnsignedChunk[T] `serialize:"true"`
	Signer           [bls.PublicKeyLen]byte `serialize:"true"`
	Signature        [bls.SignatureLen]byte `serialize:"true"`

	bytes []byte
	id    ids.ID
}

func (c *Chunk[T]) init() error {
	packer := wrappers.Packer{Bytes: make([]byte, 0, InitialChunkSize), MaxSize: consts.NetworkSizeLimit}
	if err := codec.LinearCodec.MarshalInto(c, &packer); err != nil {
		return err
	}

	c.bytes = packer.Bytes
	c.id = utils.ToID(c.bytes)

	return nil
}

func signChunk[T Tx](
	chunk UnsignedChunk[T],
	sk *bls.SecretKey,
	pk *bls.PublicKey,
	networkID uint32,
	chainID ids.ID,
) (Chunk[T], error) {
	packer := wrappers.Packer{Bytes: make([]byte, 0, InitialChunkSize), MaxSize: consts.NetworkSizeLimit}
	if err := codec.LinearCodec.MarshalInto(chunk, &packer); err != nil {
		return Chunk[T]{}, err
	}

	msg, err := warp.NewUnsignedMessage(networkID, chainID, packer.Bytes)
	if err != nil {
		return Chunk[T]{}, err
	}

	signer := warp.NewSigner(sk, networkID, chainID)
	signatureBytes, err := signer.Sign(msg)
	if err != nil {
		return Chunk[T]{}, err
	}

	pkBytes := [bls.PublicKeyLen]byte{}
	signature := [bls.SignatureLen]byte{}

	copy(pkBytes[:], bls.PublicKeyToCompressedBytes(pk))
	copy(signature[:], signatureBytes)

	return newChunk(chunk, pkBytes, signature)
}

// newChunk signs a chunk
func newChunk[T Tx](
	unsignedChunk UnsignedChunk[T],
	signer [bls.PublicKeyLen]byte,
	signature [bls.SignatureLen]byte,
) (Chunk[T], error) {
	c := Chunk[T]{
		UnsignedChunk: unsignedChunk,
		Signer:        signer,
		Signature:     signature,
	}
	return c, c.init()
}

func ParseChunk[T Tx](chunkBytes []byte) (Chunk[T], error) {
	c := Chunk[T]{}
	if err := codec.LinearCodec.UnmarshalFrom(&wrappers.Packer{Bytes: chunkBytes}, &c); err != nil {
		return Chunk[T]{}, err
	}

	return c, c.init()
}

type Block struct {
	ParentID  ids.ID `serialize:"true"`
	Height    uint64 `serialize:"true"`
	Timestamp int64  `serialize:"true"`

	ChunkCerts []*ChunkCertificate `serialize:"true"`

	blkID    ids.ID
	blkBytes []byte
}

func (b Block) GetID() ids.ID {
	return b.blkID
}

// TODO: implement snowman.Block interface and integrate assembler
type StatefulBlock[T Tx, B any, Result any] struct {
	handler *BlockHandler[T, B, Result]
	block   *Block
}

func (s *StatefulBlock[T, B, R]) ID() ids.ID {
	return s.block.blkID
}

func (s *StatefulBlock[T, B, R]) Parent() ids.ID {
	return s.block.ParentID
}

func (s *StatefulBlock[T, B, R]) Verify(ctx context.Context) error {
	// TODO: Verify header fields
	// TODO: de-duplicate chunk certificates (internal to block and across history)
	for _, chunkCert := range s.block.ChunkCerts {
		// TODO: verify chunks within a provided context
		if err := chunkCert.Verify(ctx, struct{}{}); err != nil {
			return err
		}
	}

	return nil
}

func (s *StatefulBlock[T, B, R]) Accept(ctx context.Context) error {
	_, err := s.handler.Accept(ctx, s.block)
	return err
}

func (*StatefulBlock[T, B, R]) Reject(context.Context) error {
	return nil
}

func (s *StatefulBlock[T, B, R]) Bytes() []byte {
	return s.block.blkBytes
}

func (s *StatefulBlock[T, B, R]) Height() uint64 {
	return s.block.Height
}

func (s *StatefulBlock[T, B, R]) Timestamp() time.Time {
	return time.Unix(0, s.block.Timestamp)
}

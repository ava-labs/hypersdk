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
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/proto/pb/dsmr"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
)

const InitialChunkSize = 250 * 1024

type UnsignedChunk[T Tx] struct {
	Producer    ids.NodeID    `serialize:"true"`
	Beneficiary codec.Address `serialize:"true"`
	Expiry      int64         `serialize:"true"`
	Txs         []T           `serialize:"true"`
}

// TODO emit configurable amount of chunks/sec
// TODO unexport?
type Chunk[T Tx] struct {
	Producer    ids.NodeID             `serialize:"true"`
	Beneficiary codec.Address          `serialize:"true"`
	Expiry      int64                  `serialize:"true"`
	Txs         []T                    `serialize:"true"`
	Signer      [bls.PublicKeyLen]byte `serialize:"true"`
	Signature   [bls.SignatureLen]byte `serialize:"true"`

	bytes []byte
	//TODO export?
	id ids.ID
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
		return Chunk[T]{}, nil
	}

	pkBytes := [bls.PublicKeyLen]byte{}
	signature := [bls.SignatureLen]byte{}

	copy(pkBytes[:], bls.PublicKeyToCompressedBytes(pk)[:])
	copy(signature[:], signatureBytes[:])

	return newSignedChunk(chunk, pkBytes, signature)
}

// newSignedChunk signs a chunk
func newSignedChunk[T Tx](
	unsignedChunk UnsignedChunk[T],
	signer [bls.PublicKeyLen]byte,
	signature [bls.SignatureLen]byte,
) (Chunk[T], error) {
	c := Chunk[T]{
		Producer:    unsignedChunk.Producer,
		Beneficiary: unsignedChunk.Beneficiary,
		Expiry:      unsignedChunk.Expiry,
		Txs:         unsignedChunk.Txs,
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

func marshalTxs[T Tx](txs []T) ([]*dsmr.Transaction, error) {
	result := make([]*dsmr.Transaction, 0, len(txs))

	for _, tx := range txs {
		p := &wrappers.Packer{MaxSize: consts.NetworkSizeLimit}
		if err := codec.LinearCodec.MarshalInto(tx, p); err != nil {
			return nil, err
		}

		result = append(result, &dsmr.Transaction{

			Bytes: p.Bytes,
		})
	}

	return result, nil
}

func newProtoChunk[T Tx](chunk Chunk[T]) (*dsmr.Chunk, error) {
	txs, err := marshalTxs(chunk.Txs)
	if err != nil {
		return nil, err
	}

	return &dsmr.Chunk{
		Producer:     chunk.Producer[:],
		Expiry:       chunk.Expiry,
		Beneficiary:  chunk.Beneficiary[:],
		Transactions: txs,
		Signer:       chunk.Signer[:],
		Signature:    chunk.Signature[:],
	}, nil
}

func newChunkFromProto[T Tx](chunkProto *dsmr.Chunk) (Chunk[T], error) {
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

	pkBytes := [bls.PublicKeyLen]byte{}
	signature := [bls.SignatureLen]byte{}
	copy(pkBytes[:], chunkProto.Signer[:])
	copy(signature[:], chunkProto.Signature[:])

	return newSignedChunk(
		UnsignedChunk[T]{
			Producer:    producer,
			Beneficiary: beneficiary,
			Expiry:      chunkProto.Expiry,
			Txs:         txs,
		},
		pkBytes,
		signature,
	)
}

// TODO implement
type ChunkFault struct{}

type Block struct {
	ParentID  ids.ID `serialize:"true"`
	Height    uint64 `serialize:"true"`
	Timestamp int64  `serialize:"true"`

	ChunkCerts []*ChunkCertificate `serialize:"true"`

	blkID    ids.ID
	blkBytes []byte
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

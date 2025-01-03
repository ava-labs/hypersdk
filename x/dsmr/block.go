// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//go:generate go run github.com/StephenButtolph/canoto/canoto --concurrent=false $GOFILE

package dsmr

import (
	"github.com/StephenButtolph/canoto"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/utils"
)

type Tx[T any] interface {
	canoto.FieldMaker[T]

	GetID() ids.ID
	GetExpiry() int64
	GetSponsor() codec.Address
}

type UnsignedChunk[T Tx[T]] struct {
	Producer    ids.NodeID    `canoto:"fixed bytes,1"`
	Beneficiary codec.Address `canoto:"fixed bytes,2"`
	Expiry      int64         `canoto:"int,3"`
	Txs         []T           `canoto:"repeated field,4"`

	canotoData canotoData_UnsignedChunk
}

// TODO emit configurable amount of chunks/sec
type Chunk[T Tx[T]] struct {
	UnsignedChunk[T] `canoto:"value,1"`
	Signer           [bls.PublicKeyLen]byte `canoto:"fixed bytes,2"`
	Signature        [bls.SignatureLen]byte `canoto:"fixed bytes,3"`

	canotoData canotoData_Chunk

	bytes []byte
	id    ids.ID
}

func (c *Chunk[T]) init() {
	c.bytes = c.MarshalCanoto()
	c.id = utils.ToID(c.bytes)
}

func signChunk[T Tx[T]](
	chunk UnsignedChunk[T],
	networkID uint32,
	chainID ids.ID,
	pk *bls.PublicKey,
	signer warp.Signer,
) (Chunk[T], error) {
	chunkBytes := chunk.MarshalCanoto()
	msg, err := warp.NewUnsignedMessage(networkID, chainID, chunkBytes)
	if err != nil {
		return Chunk[T]{}, err
	}

	signatureBytes, err := signer.Sign(msg)
	if err != nil {
		return Chunk[T]{}, err
	}

	pkBytes := [bls.PublicKeyLen]byte{}
	signature := [bls.SignatureLen]byte{}

	copy(pkBytes[:], bls.PublicKeyToCompressedBytes(pk))
	copy(signature[:], signatureBytes)

	return newChunk(chunk, pkBytes, signature), nil
}

// newChunk signs a chunk
func newChunk[T Tx[T]](
	unsignedChunk UnsignedChunk[T],
	signer [bls.PublicKeyLen]byte,
	signature [bls.SignatureLen]byte,
) Chunk[T] {
	c := Chunk[T]{
		UnsignedChunk: unsignedChunk,
		Signer:        signer,
		Signature:     signature,
	}
	c.init()
	return c
}

func ParseChunk[T Tx[T]](chunkBytes []byte) (Chunk[T], error) {
	var c Chunk[T]
	if err := c.UnmarshalCanoto(chunkBytes); err != nil {
		return Chunk[T]{}, err
	}
	c.init()
	return c, nil
}

type BlockHeader struct {
	ParentID  ids.ID `canoto:"fixed bytes,1"`
	Height    uint64 `canoto:"int,2"`
	Timestamp int64  `canoto:"int,3"`

	canotoData canotoData_BlockHeader
}

type Block struct {
	BlockHeader
	ChunkCerts []*ChunkCertificate `canoto:"repeated pointer,1"`

	canotoData canotoData_Block

	blkID    ids.ID
	blkBytes []byte
}

func (b Block) GetID() ids.ID {
	return b.blkID
}

// ExecutedBlock contains block data with any referenced chunks reconstructed
type ExecutedBlock[T Tx[T]] struct {
	BlockHeader
	ID     ids.ID
	Chunks []Chunk[T] `canoto:"repeated value,1"`

	canotoData canotoData_ExecutedBlock
}

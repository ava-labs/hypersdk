// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
)

const InitialChunkSize = 250 * 1024

var ErrFailedChunkSigVerification = errors.New("failed to verify bls chunk signature")

type Tx interface {
	GetID() ids.ID
	GetExpiry() int64
	GetSponsor() codec.Address
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
	networkID uint32,
	chainID ids.ID,
	pk *bls.PublicKey,
	signer warp.Signer,
) (Chunk[T], error) {
	packer := wrappers.Packer{Bytes: make([]byte, 0, InitialChunkSize), MaxSize: consts.NetworkSizeLimit}
	if err := codec.LinearCodec.MarshalInto(chunk, &packer); err != nil {
		return Chunk[T]{}, err
	}

	msg, err := warp.NewUnsignedMessage(networkID, chainID, packer.Bytes)
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

	return newChunk(chunk, pkBytes, signature)
}

func (c *Chunk[T]) Verify(networkID uint32, chainID ids.ID) error {
	signature, err := bls.SignatureFromBytes(c.Signature[:])
	if err != nil {
		return err
	}

	pk, err := bls.PublicKeyFromCompressedBytes(c.Signer[:])
	if err != nil {
		return err
	}

	// Construct the unsigned message from the UnsignedChunk (stripping the signature fields)
	packer := wrappers.Packer{Bytes: make([]byte, 0, InitialChunkSize), MaxSize: consts.NetworkSizeLimit}
	if err := codec.LinearCodec.MarshalInto(c.UnsignedChunk, &packer); err != nil {
		return err
	}

	msg, err := warp.NewUnsignedMessage(networkID, chainID, packer.Bytes)
	if err != nil {
		return fmt.Errorf("failed to create unsigned warp message from chunk: %w", err)
	}

	if !bls.Verify(pk, signature, msg.Bytes()) {
		return ErrFailedChunkSigVerification
	}

	return nil
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

// validityWindowBlock implements the validity window's execution block interface
type validityWindowBlock struct {
	Block
	certs      set.Set[ids.ID]
	chunkCerts []*emapChunkCertificate
}

func (e validityWindowBlock) Contains(id ids.ID) bool {
	return e.certs.Contains(id)
}

func (e validityWindowBlock) GetContainers() []*emapChunkCertificate {
	return e.chunkCerts
}

func NewValidityWindowBlock(innerBlock Block) validityWindowBlock {
	certSet := set.Set[ids.ID]{}
	chunkCerts := make([]*emapChunkCertificate, len(innerBlock.ChunkCerts))
	for i, c := range innerBlock.ChunkCerts {
		certSet.Add(c.ChunkID)
		chunkCerts[i] = &emapChunkCertificate{*c}
	}
	return validityWindowBlock{
		Block:      innerBlock,
		certs:      certSet,
		chunkCerts: chunkCerts,
	}
}

type BlockHeader struct {
	ParentID  ids.ID `serialize:"true"`
	Height    uint64 `serialize:"true"`
	Timestamp int64  `serialize:"true"`
}

type Block struct {
	BlockHeader
	ChunkCerts []*ChunkCertificate `serialize:"true"`

	blkID    ids.ID
	blkBytes []byte
}

func (b Block) GetID() ids.ID {
	return b.blkID
}

func (b Block) GetTimestamp() int64 {
	return b.Timestamp
}

func (b Block) GetHeight() uint64 {
	return b.Height
}

func (b Block) GetParent() ids.ID {
	return b.ParentID
}

// ExecutedBlock contains block data with any referenced chunks reconstructed
type ExecutedBlock[T Tx] struct {
	BlockHeader
	ID     ids.ID
	Chunks []Chunk[T] `serialize:"true"`
}

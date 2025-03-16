// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"errors"
	"fmt"

	"github.com/StephenButtolph/canoto"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/bls"
	"github.com/ava-labs/hypersdk/internal/emap"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	_ canoto.Field                          = (*Chunk)(nil)
	_ emap.Item                             = (*EChunk)(nil)
	_ validitywindow.ExecutionBlock[EChunk] = (*EChunkBlock)(nil)

	ErrNilChunkCert      = errors.New("nil chunk cert in block")
	ErrNilChunkReference = errors.New("nil chunk reference")
)

type Chunk struct {
	Builder ids.NodeID `canoto:"fixed bytes,1"`
	Expiry  int64      `canoto:"int,2"`
	Data    []byte     `canoto:"bytes,3"`

	bytes []byte
	id    ids.ID

	canotoData canotoData_Chunk
}

func NewChunk(
	builder ids.NodeID,
	expiry int64,
	data []byte,
) *Chunk {
	unsignedChunk := &Chunk{
		Builder: builder,
		Expiry:  expiry,
		Data:    data,
	}
	unsignedChunk.bytes = unsignedChunk.MarshalCanoto()
	unsignedChunk.id = utils.ToID(unsignedChunk.bytes)
	return unsignedChunk
}

func ParseUnsignedChunk(bytes []byte) (*Chunk, error) {
	unsignedChunk := &Chunk{}
	if err := unsignedChunk.UnmarshalCanoto(bytes); err != nil {
		return nil, err
	}
	unsignedChunk.CalculateCanotoCache()
	unsignedChunk.bytes = bytes
	unsignedChunk.id = utils.ToID(unsignedChunk.bytes)
	return unsignedChunk, nil
}

func (u *Chunk) CreateReference() *ChunkReference {
	return NewChunkReference(u.id, u.Builder, u.Expiry)
}

type ChunkReference struct {
	ChunkID ids.ID     `canoto:"fixed bytes,1"`
	Builder ids.NodeID `canoto:"fixed bytes,2"`
	Expiry  int64      `canoto:"int,3"`

	canotoData canotoData_ChunkReference

	bytes []byte
}

func NewChunkReference(
	chunkID ids.ID,
	builder ids.NodeID,
	expiry int64,
) *ChunkReference {
	chunkRef := &ChunkReference{
		ChunkID: chunkID,
		Builder: builder,
		Expiry:  expiry,
	}
	chunkRef.bytes = chunkRef.MarshalCanoto()
	return chunkRef
}

func ParseChunkReference(bytes []byte) (*ChunkReference, error) {
	chunkRef := new(ChunkReference)
	if err := chunkRef.UnmarshalCanoto(bytes); err != nil {
		return chunkRef, err
	}
	chunkRef.bytes = bytes
	return chunkRef, nil
}

type SerializeChunkCertificate struct {
	Reference *ChunkReference `canoto:"pointer,1"`
	Signature []byte          `canoto:"bytes,2"`

	canotoData canotoData_SerializeChunkCertificate
}

type ChunkCertificate struct {
	Reference *ChunkReference
	Signature *warp.BitSetSignature

	bytes []byte
}

func NewChunkCertificate(chunkRef *ChunkReference, signature *warp.BitSetSignature) (*ChunkCertificate, error) {
	chunkCert := &ChunkCertificate{
		Reference: chunkRef,
		Signature: signature,
	}

	signaturePacker := &wrappers.Packer{
		Bytes:   make([]byte, 0, bls.SignatureLen+100),
		MaxSize: 1_000, // XXX
	}
	if err := codec.LinearCodec.MarshalInto(chunkCert.Signature, signaturePacker); err != nil {
		return nil, err
	}
	canotoChunkCert := SerializeChunkCertificate{
		Reference: chunkRef,
		Signature: signaturePacker.Bytes,
	}
	chunkCert.bytes = canotoChunkCert.MarshalCanoto()
	return chunkCert, nil
}

func ParseChunkCertificate(bytes []byte) (*ChunkCertificate, error) {
	chunkCert := &ChunkCertificate{}
	if err := chunkCert.UnmarshalCanotoFrom(canoto.Reader{B: bytes}); err != nil {
		return nil, err
	}
	if chunkCert.Reference == nil {
		return nil, ErrNilChunkReference
	}
	chunkCert.bytes = bytes
	return chunkCert, nil
}

func (d *ChunkCertificate) MarshalCanotoInto(w canoto.Writer) canoto.Writer {
	canoto.Append(&w, d.bytes)
	return w
}

func (d *ChunkCertificate) CalculateCanotoCache() {}

func (d *ChunkCertificate) CachedCanotoSize() int {
	return len(d.bytes)
}

func (d *ChunkCertificate) UnmarshalCanotoFrom(r canoto.Reader) error {
	serializeCert := &SerializeChunkCertificate{}
	if err := serializeCert.UnmarshalCanotoFrom(r); err != nil {
		return err
	}

	if err := codec.LinearCodec.UnmarshalFrom(
		&wrappers.Packer{Bytes: serializeCert.Signature},
		&d.Signature,
	); err != nil {
		return err
	}
	d.Reference = serializeCert.Reference
	d.bytes = r.B
	return nil
}

func (d *ChunkCertificate) ValidCanoto() bool {
	if !d.Reference.ValidCanoto() {
		return false
	}
	if d.Signature == nil {
		return false
	}
	return true
}

func (cr *ChunkReference) String() string {
	return fmt.Sprintf("ChunkReference(ChunkID: %s, Builder: %s, Expiry: %d)",
		cr.ChunkID,
		cr.Builder,
		cr.Expiry,
	)
}

type Block struct {
	ParentID     ids.ID              `canoto:"fixed bytes,1"`
	Height       uint64              `canoto:"int,2"`
	Timestamp    int64               `canoto:"int,3"`
	Chunks       []*ChunkCertificate `canoto:"repeated pointer,4"`
	BlockContext *block.Context      `canoto:"pointer,5"`

	canotoData canotoData_Block

	bytes []byte
	id    ids.ID
}

func NewBlock(
	parentID ids.ID,
	height uint64,
	timestamp int64,
	chunkCerts []*ChunkCertificate,
	blockContext *block.Context,
) *Block {
	block := &Block{
		ParentID:     parentID,
		Height:       height,
		Timestamp:    timestamp,
		Chunks:       chunkCerts,
		BlockContext: blockContext,
	}
	block.bytes = block.MarshalCanoto()
	block.id = utils.ToID(block.bytes)
	return block
}

func ParseBlock(bytes []byte) (*Block, error) {
	block := &Block{}
	if err := block.UnmarshalCanoto(bytes); err != nil {
		return nil, err
	}
	for i, chunkCert := range block.Chunks {
		if chunkCert == nil {
			return nil, fmt.Errorf("%w: %d", ErrNilChunkCert, i)
		}
		chunkCert.CalculateCanotoCache()
	}
	block.CalculateCanotoCache()
	block.bytes = bytes
	block.id = utils.ToID(block.bytes)
	return block, nil
}

// Implement snow.Block
func (b *Block) String() string {
	return fmt.Sprintf("Block(ID: %s, ParentID: %s, Height: %d, Timestamp: %d, NumChunks: %d)",
		b.id,
		b.ParentID,
		b.Height,
		b.Timestamp,
		len(b.Chunks),
	)
}
func (b *Block) GetID() ids.ID              { return b.id }
func (b *Block) GetParent() ids.ID          { return b.ParentID }
func (b *Block) GetTimestamp() int64        { return b.Timestamp }
func (b *Block) GetBytes() []byte           { return b.bytes }
func (b *Block) GetHeight() uint64          { return b.Height }
func (b *Block) GetContext() *block.Context { return b.BlockContext }

type Parser struct{}

func (Parser) ParseBlock(ctx context.Context, bytes []byte) (*Block, error) {
	return ParseBlock(bytes)
}

type EChunk struct {
	chunkID ids.ID
	expiry  int64
}

func (e EChunk) GetID() ids.ID {
	return e.chunkID
}

func (e EChunk) GetExpiry() int64 {
	return e.expiry
}

type EChunkBlock struct {
	*Block

	eChunks    []EChunk
	eChunksSet set.Set[ids.ID]
}

func newEChunkBlock(b *Block) *EChunkBlock {
	eChunks := make([]EChunk, len(b.Chunks))
	eChunksSet := set.NewSet[ids.ID](len(b.Chunks))
	for i, cert := range b.Chunks {
		eChunks[i] = EChunk{
			chunkID: cert.Reference.ChunkID,
			expiry:  cert.Reference.Expiry,
		}
		eChunksSet.Add(cert.Reference.ChunkID)
	}
	return &EChunkBlock{
		Block:      b,
		eChunks:    eChunks,
		eChunksSet: eChunksSet,
	}
}

func (e *EChunkBlock) GetBytes() []byte {
	return e.Block.bytes
}

func (e *EChunkBlock) GetID() ids.ID {
	return e.id
}

func (e *EChunkBlock) GetParent() ids.ID {
	return e.ParentID
}

func (e *EChunkBlock) GetTimestamp() int64 {
	return e.Timestamp
}

func (e *EChunkBlock) GetHeight() uint64 {
	return e.Height
}

func (e *EChunkBlock) GetContainers() []EChunk {
	return e.eChunks
}

func (e *EChunkBlock) Contains(id ids.ID) bool {
	return e.eChunksSet.Contains(id)
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
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
	_ emap.Item                             = (*eChunk)(nil)
	_ validitywindow.ExecutionBlock[eChunk] = (*eChunkBlock)(nil)
)

type (
	Rules interface {
		GetValidityWindow() int64
		GetMaxPendingBandwidthPerValidator() uint64
	}

	RuleFactory interface {
		GetRules(timestamp int64) Rules
	}

	UnsignedChunk struct {
		Builder ids.NodeID `canoto:"fixed bytes,1"`
		Expiry  int64      `canoto:"int,2"`
		Data    []byte     `canoto:"bytes,3"`

		bytes []byte
		id    ids.ID

		canotoData canotoData_UnsignedChunk
	}

	ChunkReference struct {
		ChunkID ids.ID     `canoto:"fixed bytes,1"`
		Builder ids.NodeID `canoto:"fixed bytes,2"`
		Expiry  int64      `canoto:"int,3"`

		canotoData canotoData_ChunkReference

		bytes []byte
	}

	SerializeChunkCertificate struct {
		Reference ChunkReference `canoto:"value,1"`
		Signature []byte         `canoto:"bytes,2"`

		canotoData canotoData_SerializeChunkCertificate
	}

	ChunkCertificate struct {
		Reference ChunkReference
		Signature *warp.BitSetSignature

		bytes []byte
	}

	Chunk struct {
		UnsignedChunk UnsignedChunk    `canoto:"value,1"`
		Certificate   ChunkCertificate `canoto:"value,2"`

		canotoData canotoData_Chunk
	}

	Block struct {
		ParentID     ids.ID             `canoto:"fixed bytes,1"`
		Height       uint64             `canoto:"int,2"`
		Timestamp    int64              `canoto:"int,3"`
		Chunks       []ChunkCertificate `canoto:"repeated value,4"`
		BlockContext *block.Context     `canoto:"pointer,5"`

		canotoData canotoData_Block

		bytes []byte
		id    ids.ID
	}
)

func NewUnsignedChunk(
	builder ids.NodeID,
	expiry int64,
	data []byte,
) *UnsignedChunk {
	unsignedChunk := &UnsignedChunk{
		Builder: builder,
		Expiry:  expiry,
		Data:    data,
	}
	unsignedChunk.bytes = unsignedChunk.MarshalCanoto()
	unsignedChunk.id = utils.ToID(unsignedChunk.bytes)
	return unsignedChunk
}

func ParseUnsignedChunk(bytes []byte) (*UnsignedChunk, error) {
	unsignedChunk := &UnsignedChunk{}
	if err := unsignedChunk.UnmarshalCanoto(bytes); err != nil {
		return nil, err
	}
	unsignedChunk.CalculateCanotoCache()
	unsignedChunk.bytes = bytes
	unsignedChunk.id = utils.ToID(unsignedChunk.bytes)
	return unsignedChunk, nil
}

func (u UnsignedChunk) Reference() ChunkReference {
	cr := ChunkReference{
		ChunkID: u.id,
		Builder: u.Builder,
		Expiry:  u.Expiry,
	}

	cr.bytes = cr.MarshalCanoto()
	return cr
}

func NewChunkCertificate(chunkRef ChunkReference, signature *warp.BitSetSignature) (*ChunkCertificate, error) {
	chunkCert := &ChunkCertificate{
		Reference: chunkRef,
		Signature: signature,
	}

	signaturePacker := &wrappers.Packer{
		Bytes: make([]byte, 0, bls.SignatureLen+100),
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

func (cr ChunkReference) String() string {
	return fmt.Sprintf("ChunkReference(ChunkID: %s, Builder: %s, Expiry: %d)",
		cr.ChunkID,
		cr.Builder,
		cr.Expiry,
	)
}

func NewBlock(
	parentID ids.ID,
	height uint64,
	timestamp int64,
	chunkCerts []ChunkCertificate,
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

type eChunk struct {
	chunkID ids.ID
	expiry  int64
}

func (e eChunk) GetID() ids.ID {
	return e.chunkID
}

func (e eChunk) GetExpiry() int64 {
	return e.expiry
}

type eChunkBlock struct {
	*Block

	eChunks    []eChunk
	eChunksSet set.Set[ids.ID]
}

func newEChunkBlock(b *Block) *eChunkBlock {
	eChunks := make([]eChunk, len(b.Chunks))
	eChunksSet := set.NewSet[ids.ID](len(b.Chunks))
	for i, cert := range b.Chunks {
		eChunks[i] = eChunk{
			chunkID: cert.Reference.ChunkID,
			expiry:  cert.Reference.Expiry,
		}
		eChunksSet.Add(cert.Reference.ChunkID)
	}
	return &eChunkBlock{
		Block:      b,
		eChunks:    eChunks,
		eChunksSet: eChunksSet,
	}
}

func (e *eChunkBlock) GetID() ids.ID {
	return e.id
}

func (e *eChunkBlock) GetParent() ids.ID {
	return e.ParentID
}

func (e *eChunkBlock) GetTimestamp() int64 {
	return e.Timestamp
}

func (e *eChunkBlock) GetHeight() uint64 {
	return e.Height
}

func (e *eChunkBlock) GetContainers() []eChunk {
	return e.eChunks
}

func (e *eChunkBlock) Contains(id ids.ID) bool {
	return e.eChunksSet.Contains(id)
}

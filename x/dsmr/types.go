// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"github.com/StephenButtolph/canoto"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/utils"
)

var _ canoto.Field = (*Chunk)(nil)

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
		ParentID     ids.ID         `canoto:"fixed bytes,1"`
		Height       uint64         `canoto:"int,2"`
		Timestamp    int64          `canoto:"int,3"`
		Chunks       []Chunk        `canoto:"repeated value,4"`
		BlockContext *block.Context `canoto:"pointer,5"`

		canotoData canotoData_Block
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

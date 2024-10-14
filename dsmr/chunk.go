// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/wrappers"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
)

const InitialChunkSize = 250 * 1024

type Chunk[T Tx] struct {
	Producer    ids.NodeID    `serialize:"true"`
	Expiry      int64         `serialize:"true"`
	Beneficiary codec.Address `serialize:"true"`
	//TODO define serialization
	Txs       []T           `serialize:"true"`
	Signer    bls.PublicKey `serialize:"true"`
	Signature bls.Signature `serialize:"true"`

	bytes []byte
	id    ids.ID
}

func NewChunk[T Tx](txs []T, expiry int64) (Chunk[T], error) {
	c := Chunk[T]{
		Txs:    txs,
		Expiry: expiry,
	}

	packer := wrappers.Packer{Bytes: make([]byte, 0, InitialChunkSize), MaxSize: consts.NetworkSizeLimit}
	if err := codec.LinearCodec.MarshalInto(c, &packer); err != nil {
		return Chunk[T]{}, err
	}
	c.bytes = packer.Bytes
	c.id = utils.ToID(c.bytes)
	return c, nil
}

func ParseChunk[T Tx](chunkBytes []byte) (*Chunk[T], error) {
	c := &Chunk[T]{}
	if err := codec.LinearCodec.UnmarshalFrom(&wrappers.Packer{Bytes: chunkBytes}, c); err != nil {
		return nil, err
	}
	c.bytes = chunkBytes
	c.id = utils.ToID(c.bytes)
	return c, nil
}

func (c *Chunk[T]) ID() ids.ID {
	return c.id
}

func (c *Chunk[T]) Bytes() []byte {
	return c.bytes
}

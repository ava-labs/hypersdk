// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"time"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/network/p2p"
)

func New[T Tx](getChunkClient *p2p.Client) (*Node[T], error) {
	storage, err := newChunkStorage[T](&NoVerifier[T]{}, memdb.New())
	if err != nil {
		return nil, err
	}

	return &Node[T]{
		GetChunkHandler: &GetChunkHandler[T]{
			storage: storage,
		},
		GetChunkSignatureHandler: &GetChunkSignatureHandler[T]{
			storage: storage,
		},
		client: GetChunkClient[T]{
			client: getChunkClient,
		},
		storage: storage,
		chunks:  make(chan Chunk[T], 1),
	}, nil
}

type Node[T Tx] struct {
	GetChunkHandler          *GetChunkHandler[T]
	GetChunkSignatureHandler *GetChunkSignatureHandler[T]

	//TODO chunk handler
	client           GetChunkClient[T]
	chunkCertBuilder chunkCertBuilder[T]
	blockBuilder     blockBuilder[T]
	storage          *chunkStorage[T]

	chunks chan Chunk[T]
}

// BuildChunk adds a chunk to the node with the provided transactions
// TODO why return error
// TODO handle frozen sponsor + validator assignments
func (n Node[T]) BuildChunk(txs []T, expiry time.Time) (Chunk[T], error) {
	chunk, err := newChunk[T](txs, expiry)
	if err != nil {
		return Chunk[T]{}, nil
	}

	cert := &ChunkCertificate{
		ChunkID:   chunk.id,
		Expiry:    chunk.Expiry,
		Signature: NoVerifyChunkSignature{},
	}

	return chunk, n.storage.AddLocalChunkWithCert(chunk, cert)
}

// NewBlock TODO should we quiesce
func (n Node[T]) NewBlock() (Block, error) {
	return Block{
		Chunks: n.storage.GatherChunkCerts(),
	}, nil
}

// consumes chunks and aggregates signtures to generate chunk certs
type chunkCertBuilder[T Tx] struct {
	client *p2p.Client
}

// TODO implement
func (c *chunkCertBuilder[T]) NewCert(chunk Chunk[T]) (ChunkCertificate, error) {
	return ChunkCertificate{}, nil
}

// TODO can this share impl w/ chunkBuilder?
type blockBuilder[T Tx] struct{}

// Add returns if a block was built
func (b *blockBuilder[T]) Add(chunk ChunkCertificate) (Block, bool) {
	return Block{}, true
}

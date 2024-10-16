// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/network/p2p"
)

func New[T Tx](
	getChunkClient *p2p.Client,
	txsPerChunk int,
) *Node[T] {
	return &Node[T]{
		client: GetChunkClient[T]{
			client: getChunkClient,
		},
		chunkBuilder: chunkBuilder[T]{
			threshold: txsPerChunk,
		},
		chunks: make(chan Chunk[T], 1),
	}
}

type Node[T Tx] struct {
	GetChunkHandler
	GetChunkSignatureHandler[T]
	//TODO chunk handler
	client           GetChunkClient[T]
	chunkBuilder     chunkBuilder[T]
	chunkCertBuilder chunkCertBuilder[T]
	blockBuilder     blockBuilder[T]

	chunks chan Chunk[T]
}

func (n Node[_]) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case chunk := <-n.chunks:
			_, err := n.chunkCertBuilder.NewCert(chunk)
			if err != nil {
				return fmt.Errorf("failed to generate chunk cert: %w", err)
			}

			//TODO aggregate chunk cert
		}
	}
}

// TODO why return error
// TODO handle frozen sponsor + validator assignments
// Caller is assumed to de-dup transactions?
func (n Node[T]) AddTx(tx T) error {
	chunk, err := n.chunkBuilder.Add(tx, 0)
	if err != nil {
		return err
	}

	n.chunks <- chunk
	return nil
}

// NewBlock TODO should we quiesce
func (n Node[T]) NewBlock() (Block, error) {
	return Block{}, nil
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

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package dsmr

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"
)

// Note: Assembler breaks assembling and executing a block into two steps
// but these will be called one after the other.
type Assembler[T Tx, Block any, Result any] interface {
	AssembleBlock(ctx context.Context, parentBlock Block, timestamp int64, blockHeight uint64, txs []T) (Block, error)
	ExecuteBlock(ctx context.Context, b Block) (Result, error)
}

type ChunkGatherer[T Tx] interface {
	// CollectChunks gathers the corresponding chunks and writes any chunks to
	// storage that were not already persisted.
	CollectChunks(chunkCerts []*ChunkCertificate) ([]*Chunk[T], error)
}

type BlockHandler[T Tx, Block any, Result any] struct {
	lastAcceptedBlock Block
	chunkGatherer     ChunkGatherer[T]
	Assembler         Assembler[T, Block, Result]
}

func (b *BlockHandler[T, B, R]) Accept(ctx context.Context, block *Block) (R, error) {
	// Collect and store chunks in the accepted block
	chunks, err := b.chunkGatherer.CollectChunks(block.Chunks)
	if err != nil {
		return *new(R), err
	}

	// Collect and de-duplicate txs
	numTxs := 0
	for _, chunk := range chunks {
		numTxs += len(chunk.Txs)
	}

	txs := make([]T, 0, numTxs)
	txSet := set.Set[ids.ID]{}
	for _, chunk := range chunks {
		for _, tx := range chunk.Txs {
			txID := tx.GetID()
			if txSet.Contains(txID) {
				continue
			}
			txSet.Add(txID)
			txs = append(txs, tx)
		}
	}

	// Assemble and execute the block
	assembledBlk, err := b.Assembler.AssembleBlock(ctx, b.lastAcceptedBlock, block.Timestamp, block.Height+1, txs)
	if err != nil {
		return *new(R), err
	}
	b.lastAcceptedBlock = assembledBlk
	return b.Assembler.ExecuteBlock(ctx, assembledBlk)
}

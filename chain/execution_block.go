// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/internal/workers"
)

type ExecutionBlock struct {
	*StatelessBlock

	// The below fields are only populated for blocks that may have Verify
	// called on them. This excludes the genesis block and any block built
	// locally.
	sigJob workers.Job
	txsSet set.Set[ids.ID]

	// authCounts can be used by batch signature verification
	// to preallocate memory
	authCounts map[uint8]int
}

func NewExecutionBlockNoVerify(block *StatelessBlock) *ExecutionBlock {
	return &ExecutionBlock{
		StatelessBlock: block,
	}
}

func NewExecutionBlock(
	ctx context.Context,
	block *StatelessBlock,
	c *Chain,
	populateTxs bool,
) (*ExecutionBlock, error) {
	eb := &ExecutionBlock{
		StatelessBlock: block,
		txsSet:         set.NewSet[ids.ID](len(block.Txs)),
		authCounts:     make(map[uint8]int),
	}
	if !populateTxs {
		return eb, nil
	}

	ctx, span := c.tracer.Start(ctx, "NewExecutionBlock.populateTxs")
	defer span.End()

	for _, tx := range block.Txs {
		if eb.txsSet.Contains(tx.ID()) {
			return nil, ErrDuplicateTx
		}
		eb.txsSet.Add(tx.ID())
		eb.authCounts[tx.Auth.GetTypeID()]++
	}
	sigJob, err := c.authVerificationWorkers.NewJob(len(block.Txs))
	if err != nil {
		return nil, err
	}
	eb.sigJob = sigJob

	// Setup signature verification job
	_, sigVerifySpan := c.tracer.Start(ctx, "NewExecutionBlock.verifySignatures") //nolint:spancheck

	batchVerifier := NewAuthBatch(c.authVM, sigJob, eb.authCounts)
	// Make sure to always call [Done], otherwise we will block all future [Workers]
	defer func() {
		// BatchVerifier is given the responsibility to call [b.sigJob.Done()] because it may add things
		// to the work queue async and that may not have completed by this point.
		go batchVerifier.Done(func() { sigVerifySpan.End() })
	}()

	for _, tx := range block.Txs {
		unsignedTxBytes, err := tx.UnsignedBytes()
		if err != nil {
			return nil, err
		}
		batchVerifier.Add(unsignedTxBytes, tx.Auth)
	}
	return eb, nil
}

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

	// authCounts can be used by batch signature verification
	// to preallocate memory
	authCounts map[uint8]int
	txsSet     set.Set[ids.ID]
	sigJob     workers.Job
}

func NewExecutionBlock(block *StatelessBlock) *ExecutionBlock {
	return &ExecutionBlock{
		StatelessBlock: block,
	}
}

func (b *ExecutionBlock) initTxs() error {
	if b.txsSet.Len() == len(b.Txs) {
		return nil
	}
	b.authCounts = make(map[uint8]int)
	b.txsSet = set.NewSet[ids.ID](len(b.Txs))
	for _, tx := range b.Txs {
		if b.txsSet.Contains(tx.ID()) {
			return ErrDuplicateTx
		}
		b.txsSet.Add(tx.ID())
		b.authCounts[tx.Auth.GetTypeID()]++
	}

	return nil
}

// AsyncVerify starts async signature verification as early as possible
func (c *Chain) AsyncVerify(ctx context.Context, block *ExecutionBlock) error {
	ctx, span := c.tracer.Start(ctx, "Chain.AsyncVerify")
	defer span.End()

	if err := block.initTxs(); err != nil {
		return err
	}
	if block.sigJob != nil {
		return nil
	}
	sigJob, err := c.authVerificationWorkers.NewJob(len(block.Txs))
	if err != nil {
		return err
	}
	block.sigJob = sigJob

	// Setup signature verification job
	_, sigVerifySpan := c.tracer.Start(ctx, "NewExecutionBlock.verifySignatures") //nolint:spancheck

	batchVerifier := NewAuthBatch(c.authVM, sigJob, block.authCounts)
	// Make sure to always call [Done], otherwise we will block all future [Workers]
	defer func() {
		// BatchVerifier is given the responsibility to call [b.sigJob.Done()] because it may add things
		// to the work queue async and that may not have completed by this point.
		go batchVerifier.Done(func() { sigVerifySpan.End() })
	}()

	for _, tx := range block.Txs {
		unsignedTxBytes, err := tx.UnsignedBytes()
		if err != nil {
			return err //nolint:spancheck
		}
		batchVerifier.Add(unsignedTxBytes, tx.Auth)
	}
	return nil
}

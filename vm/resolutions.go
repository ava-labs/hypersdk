// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/internal/gossiper"
	"github.com/ava-labs/hypersdk/internal/validators"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
)

var (
	_ gossiper.ValidatorSet                         = (*VM)(nil)
	_ validators.Backend                            = (*VM)(nil)
	_ validitywindow.ChainIndex[*chain.Transaction] = (*VM)(nil)
	_ chain.AuthVM                                  = (*VM)(nil)
	_ ConsensusAPI                                  = (*VM)(nil)
	_ Submitter                                     = (*VM)(nil)
)

// validitywindow.ChainIndex[*chain.Transaction]
func (vm *VM) GetExecutionBlock(ctx context.Context, blkID ids.ID) (validitywindow.ExecutionBlock[*chain.Transaction], error) {
	_, span := vm.tracer.Start(ctx, "VM.GetExecutionBlock")
	defer span.End()

	blk, err := vm.consensusIndex.GetBlock(ctx, blkID)
	if err != nil {
		return nil, err
	}
	return blk, nil
}

// gossiper.ValidatorSet
func (vm *VM) IsValidator(ctx context.Context, nid ids.NodeID) (bool, error) {
	return vm.proposerMonitor.IsValidator(ctx, nid)
}

func (vm *VM) Proposers(ctx context.Context, diff int, depth int) (set.Set[ids.NodeID], error) {
	return vm.proposerMonitor.Proposers(ctx, diff, depth)
}

func (vm *VM) NodeID() ids.NodeID {
	return vm.snowCtx.NodeID
}

// validators.Backend
func (vm *VM) PreferredHeight(ctx context.Context) (uint64, error) {
	blk, err := vm.consensusIndex.GetPreferredBlock(ctx)
	if err != nil {
		return 0, err
	}
	return blk.Hght, nil
}

// chain.AuthVM
func (vm *VM) Logger() logging.Logger {
	return vm.snowCtx.Log
}

func (vm *VM) GetAuthBatchVerifier(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool) {
	bv, ok := vm.authEngine[authTypeID]
	if !ok {
		return nil, false
	}
	return bv.GetBatchVerifier(cores, count), ok
}

// Submitter
func (vm *VM) Submit(
	ctx context.Context,
	txs []*chain.Transaction,
) (errs []error) {
	ctx, span := vm.tracer.Start(ctx, "VM.Submit")
	defer span.End()
	vm.metrics.txsSubmitted.Add(float64(len(txs)))

	// Create temporary execution context
	preferredBlk, err := vm.consensusIndex.GetPreferredBlock(ctx)
	if err != nil {
		return []error{err}
	}
	view := preferredBlk.View

	validTxs := []*chain.Transaction{}
	for _, tx := range txs {
		// Avoid any sig verification or state lookup if we already have tx in mempool
		txID := tx.GetID()
		if vm.Mempool.Has(ctx, txID) {
			// Don't remove from listeners, it will be removed elsewhere if not
			// included
			errs = append(errs, ErrNotAdded)
			continue
		}

		if err := vm.chain.PreExecute(ctx, preferredBlk.ExecutionBlock, view, tx); err != nil {
			errs = append(errs, err)
			continue
		}
		errs = append(errs, nil)
		validTxs = append(validTxs, tx)
	}
	vm.Mempool.Add(ctx, validTxs)
	vm.checkActivity(ctx)
	vm.metrics.mempoolSize.Set(float64(vm.Mempool.Len(ctx)))
	vm.snowCtx.Log.Info("Submitted tx(s)", zap.Int("validTxs", len(validTxs)), zap.Int("invalidTxs", len(errs)-len(validTxs)), zap.Int("mempoolSize", vm.Mempool.Len(ctx)))
	return errs
}

func (vm *VM) SubmitTx(ctx context.Context, tx *chain.Transaction) error {
	return vm.Submit(ctx, []*chain.Transaction{tx})[0]
}

// ConsensusAPI
func (vm *VM) LastAcceptedBlock(ctx context.Context) (*chain.ExecutedBlock, error) {
	blk, err := vm.consensusIndex.GetLastAccepted(ctx)
	if err != nil {
		return nil, err
	}
	return &chain.ExecutedBlock{
		Block:            blk.StatelessBlock,
		ExecutionResults: blk.ExecutionResults,
	}, nil
}

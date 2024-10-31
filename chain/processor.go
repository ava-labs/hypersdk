// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/internal/executor"
	"github.com/ava-labs/hypersdk/internal/fees"
	"github.com/ava-labs/hypersdk/internal/fetcher"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/tstate"
)

type Executor struct {
	vm VM
}

func NewExecutor(vm VM) *Executor {
	return &Executor{vm: vm}
}

func (e *Executor) Execute(
	ctx context.Context,
	b *StatefulBlock,
) ([]*Result, *fees.Manager, merkledb.View, error) {
	var (
		log = e.vm.Logger()
		r   = e.vm.Rules(b.Tmstmp)
	)

	vctx, err := b.GetVerifyContext(ctx, b.Hght, b.Prnt)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: unable to get verify context", err)
	}

	// Perform basic correctness checks before doing any expensive work
	if b.Timestamp().UnixMilli() > time.Now().Add(FutureBound).UnixMilli() {
		return nil, nil, nil, ErrTimestampTooLate
	}

	// Fetch view where we will apply block state transitions
	//
	// This call may result in our ancestry being verified.
	parentView, err := vctx.View(ctx, true)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("%w: unable to load parent view", err)
	}

	// Fetch parent height key and ensure block height is valid
	heightKey := HeightKey(e.vm.MetadataManager().HeightPrefix())
	parentHeightRaw, err := parentView.GetValue(ctx, heightKey)
	if err != nil {
		return nil, nil, nil, err
	}
	parentHeight, err := database.ParseUInt64(parentHeightRaw)
	if err != nil {
		return nil, nil, nil, err
	}
	if b.Hght != parentHeight+1 {
		return nil, nil, nil, ErrInvalidBlockHeight
	}

	// Fetch parent timestamp and confirm block timestamp is valid
	//
	// Parent may not be available (if we preformed state sync), so we
	// can't rely on being able to fetch it during verification.
	timestampKey := TimestampKey(e.vm.MetadataManager().TimestampPrefix())
	parentTimestampRaw, err := parentView.GetValue(ctx, timestampKey)
	if err != nil {
		return nil, nil, nil, err
	}
	parentTimestampUint64, err := database.ParseUInt64(parentTimestampRaw)
	if err != nil {
		return nil, nil, nil, err
	}
	parentTimestamp := int64(parentTimestampUint64)
	if b.Tmstmp < parentTimestamp+r.GetMinBlockGap() {
		return nil, nil, nil, ErrTimestampTooEarly
	}
	if len(b.Txs) == 0 && b.Tmstmp < parentTimestamp+r.GetMinEmptyBlockGap() {
		return nil, nil, nil, ErrTimestampTooEarly
	}

	if err := e.verifyExpiryReplayProtection(ctx, b, r, vctx); err != nil {
		return nil, nil, nil, err
	}

	// Compute next unit prices to use
	feeKey := FeeKey(e.vm.MetadataManager().FeePrefix())
	feeRaw, err := parentView.GetValue(ctx, feeKey)
	if err != nil {
		return nil, nil, nil, err
	}
	parentFeeManager := fees.NewManager(feeRaw)
	feeManager, err := parentFeeManager.ComputeNext(b.Tmstmp, r)
	if err != nil {
		return nil, nil, nil, err
	}

	// Process transactions
	results, ts, err := e.ExecuteTxs(ctx, b, e.vm.Tracer(), parentView, feeManager, r)
	if err != nil {
		log.Error("failed to execute block", zap.Error(err))
		return nil, nil, nil, err
	}

	// Update chain metadata
	heightKeyStr := string(heightKey)
	timestampKeyStr := string(timestampKey)
	feeKeyStr := string(feeKey)

	keys := make(state.Keys)
	keys.Add(heightKeyStr, state.Write)
	keys.Add(timestampKeyStr, state.Write)
	keys.Add(feeKeyStr, state.Write)
	tsv := ts.NewView(keys, map[string][]byte{
		heightKeyStr:    parentHeightRaw,
		timestampKeyStr: parentTimestampRaw,
		feeKeyStr:       parentFeeManager.Bytes(),
	})
	if err := tsv.Insert(ctx, heightKey, binary.BigEndian.AppendUint64(nil, b.Hght)); err != nil {
		return nil, nil, nil, err
	}
	if err := tsv.Insert(ctx, timestampKey, binary.BigEndian.AppendUint64(nil, uint64(b.Tmstmp))); err != nil {
		return nil, nil, nil, err
	}
	if err := tsv.Insert(ctx, feeKey, feeManager.Bytes()); err != nil {
		return nil, nil, nil, err
	}
	tsv.Commit()

	// Compare state root
	//
	// Because fee bytes are not recorded in state, it is sufficient to check the state root
	// to verify all fee calculations were correct.
	_, rspan := e.vm.Tracer().Start(ctx, "StatefulBlock.Verify.WaitRoot")
	start := time.Now()
	computedRoot, err := parentView.GetMerkleRoot(ctx)
	rspan.End()
	if err != nil {
		return nil, nil, nil, err
	}
	e.vm.RecordWaitRoot(time.Since(start))
	if b.StateRoot != computedRoot {
		return nil, nil, nil, fmt.Errorf(
			"%w: expected=%s found=%s",
			ErrStateRootMismatch,
			computedRoot,
			b.StateRoot,
		)
	}

	// Ensure signatures are verified
	_, sspan := e.vm.Tracer().Start(ctx, "StatefulBlock.Verify.WaitSignatures")
	start = time.Now()
	err = b.sigJob.Wait()
	sspan.End()
	if err != nil {
		return nil, nil, nil, err
	}
	e.vm.RecordWaitSignatures(time.Since(start))

	// Get view from [tstate] after processing all state transitions
	e.vm.RecordStateChanges(ts.PendingChanges())
	e.vm.RecordStateOperations(ts.OpIndex())
	view, err := ts.ExportMerkleDBView(ctx, e.vm.Tracer(), parentView)
	if err != nil {
		return nil, nil, nil, err
	}

	// Kickoff root generation
	go func() {
		start := time.Now()
		root, err := view.GetMerkleRoot(ctx)
		if err != nil {
			log.Error("merkle root generation failed", zap.Error(err))
			return
		}
		log.Info("merkle root generated",
			zap.Uint64("height", b.Hght),
			zap.Stringer("blkID", b.ID()),
			zap.Stringer("root", root),
		)
		e.vm.RecordRootCalculated(time.Since(start))
	}()
	return results, feeManager, view, nil
}

// verifyExpiryReplayProtection handles expiry replay protection
//
// Replay protection confirms a transaction has not been included within the
// past validity window.
// Before the node is ready (we have synced a validity window of blocks), this
// function may return an error when other nodes see the block as valid.
//
// If a block is already accepted, its transactions have already been added
// to the VM's seen emap and calling [IsRepeat] will return a non-zero value
// when it should already be considered valid, so we skip this step here.
func (e *Executor) verifyExpiryReplayProtection(ctx context.Context, b *StatefulBlock, rules Rules, vctx VerifyContext) error {
	if b.accepted {
		return nil
	}

	oldestAllowed := b.Tmstmp - rules.GetValidityWindow()
	if oldestAllowed < 0 {
		// Can occur if verifying genesis
		oldestAllowed = 0
	}
	dup, err := vctx.IsRepeat(ctx, oldestAllowed, b.Txs, set.NewBits(), true)
	if err != nil {
		return err
	}
	if dup.Len() > 0 {
		return fmt.Errorf("%w: duplicate in ancestry", ErrDuplicateTx)
	}
	return nil
}

type fetchData struct {
	v      []byte
	exists bool

	chunks uint16
}

func (e *Executor) ExecuteTxs(
	ctx context.Context,
	b *StatefulBlock,
	tracer trace.Tracer, //nolint:interfacer
	im state.Immutable,
	feeManager *fees.Manager,
	r Rules,
) ([]*Result, *tstate.TState, error) {
	ctx, span := tracer.Start(ctx, "Processor.Execute")
	defer span.End()

	var (
		bh     = e.vm.BalanceHandler()
		numTxs = len(b.Txs)
		t      = b.GetTimestamp()

		f          = fetcher.New(im, numTxs, e.vm.GetStateFetchConcurrency())
		txExecutor = executor.New(numTxs, e.vm.GetTransactionExecutionCores(), MaxKeyDependencies, e.vm.GetExecutorVerifyRecorder())
		ts         = tstate.New(numTxs * 2) // TODO: tune this heuristic
		results    = make([]*Result, numTxs)
	)

	// Fetch required keys and execute transactions
	for li, ltx := range b.Txs {
		i := li
		tx := ltx

		stateKeys, err := tx.StateKeys(bh)
		if err != nil {
			f.Stop()
			txExecutor.Stop()
			return nil, nil, err
		}

		// Ensure we don't consume too many units
		units, err := tx.Units(bh, r)
		if err != nil {
			f.Stop()
			txExecutor.Stop()
			return nil, nil, err
		}
		if ok, d := feeManager.Consume(units, r.GetMaxBlockUnits()); !ok {
			f.Stop()
			txExecutor.Stop()
			return nil, nil, fmt.Errorf("%w: %d too large", ErrInvalidUnitsConsumed, d)
		}

		// Prefetch state keys from disk
		txID := tx.ID()
		if err := f.Fetch(ctx, txID, stateKeys); err != nil {
			return nil, nil, err
		}
		txExecutor.Run(stateKeys, func() error {
			// Wait for stateKeys to be read from disk
			storage, err := f.Get(txID)
			if err != nil {
				return err
			}

			// Execute transaction
			//
			// It is critical we explicitly set the scope before each transaction is
			// processed
			tsv := ts.NewView(stateKeys, storage)

			// Ensure we have enough funds to pay fees
			if err := tx.PreExecute(ctx, feeManager, bh, r, tsv, t); err != nil {
				return err
			}

			result, err := tx.Execute(ctx, feeManager, bh, r, tsv, t)
			if err != nil {
				return err
			}
			results[i] = result

			// Commit results to parent [TState]
			tsv.Commit()
			return nil
		})
	}
	if err := f.Wait(); err != nil {
		return nil, nil, err
	}
	if err := txExecutor.Wait(); err != nil {
		return nil, nil, err
	}

	// Return tstate that can be used to add block-level keys to state
	return results, ts, nil
}

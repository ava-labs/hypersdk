// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	smblock "github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/math"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/tstate"
)

func HandlePreExecute(
	err error,
) (bool /* continue */, bool /* restore */, bool /* remove account */) {
	switch {
	case errors.Is(err, ErrInsufficientPrice):
		return true, true, false
	case errors.Is(err, ErrTimestampTooEarly):
		return true, true, false
	case errors.Is(err, ErrTimestampTooLate):
		return true, false, false
	case errors.Is(err, ErrInvalidBalance):
		return true, false, true
	case errors.Is(err, ErrAuthNotActivated):
		return true, false, false
	case errors.Is(err, ErrAuthFailed):
		return true, false, false
	case errors.Is(err, ErrActionNotActivated):
		return true, false, false
	default:
		// If unknown error, drop
		return true, false, false
	}
}

func BuildBlock(
	ctx context.Context,
	vm VM,
	preferred ids.ID,
	blockContext *smblock.Context,
) (*StatelessBlock, error) {
	ctx, span := vm.Tracer().Start(ctx, "chain.BuildBlock")
	defer span.End()
	log := vm.Logger()

	nextTime := time.Now().Unix()
	r := vm.Rules(nextTime)
	parent, err := vm.GetStatelessBlock(ctx, preferred)
	if err != nil {
		log.Warn("block building failed: couldn't get parent", zap.Error(err))
		return nil, err
	}
	ectx, err := GenerateExecutionContext(ctx, vm.ChainID(), nextTime, parent, vm.Tracer(), r)
	if err != nil {
		log.Warn("block building failed: couldn't get execution context", zap.Error(err))
		return nil, err
	}
	b := NewBlock(ectx, vm, parent, nextTime)

	changesEstimate := math.Min(vm.Mempool().Len(ctx), r.GetMaxBlockTxs())
	state, err := parent.childState(ctx, changesEstimate)
	if err != nil {
		log.Warn("block building failed: couldn't get parent db", zap.Error(err))
		return nil, err
	}
	stateless, err := parent.childStatelessView(ctx, r.GetMaxBlockTxs())
	if err != nil {
		log.Warn("block building failed: couldn't get stateless view", zap.Error(err))
		return nil, err
	}

	// Restorable txs after block attempt finishes
	ts := tstate.New(changesEstimate)
	b.Txs = []*Transaction{}
	var (
		oldestAllowed = nextTime - r.GetValidityWindow()

		surplusFee = uint64(0)
		mempool    = vm.Mempool()

		txsAttempted = 0
		results      = []*Result{}

		warpCount = 0

		vdrState = vm.ValidatorState()
		sm       = vm.StateManager()

		start    = time.Now()
		lockWait time.Duration
	)
	b.values = make(map[ids.ID]map[merkledb.Path]merkledb.Maybe[[]byte])
	b.nodes = make(map[ids.ID]map[merkledb.Path]merkledb.Maybe[*merkledb.Node])
	mempoolErr := mempool.Build(
		ctx,
		func(fctx context.Context, next *Transaction) (cont bool, restore bool, removeAcct bool, err error) {
			if txsAttempted == 0 {
				lockWait = time.Since(start)
			}
			txsAttempted++

			// Ensure we can process if transaction includes a warp message
			if next.WarpMessage != nil && blockContext == nil {
				log.Info(
					"dropping pending warp message because no context provided",
					zap.Stringer("txID", next.ID()),
				)
				return true, next.Base.Timestamp > oldestAllowed, false, nil
			}

			// Skip warp message if at max
			if next.WarpMessage != nil && warpCount == MaxWarpMessages {
				log.Info(
					"dropping pending warp message because already have MaxWarpMessages",
					zap.Stringer("txID", next.ID()),
				)
				return true, true, false, nil
			}

			// Check for repeats
			//
			// TODO: check a bunch at once during pre-fetch to avoid re-walking blocks
			// for every tx
			dup, err := parent.IsRepeat(ctx, oldestAllowed, []*Transaction{next})
			if err != nil {
				return false, false, false, err
			}
			if dup {
				// tx will be restored when ancestry is rejected
				return true, false, false, nil
			}

			// Ensure we have room
			nextUnits, err := next.MaxUnits(r)
			if err != nil {
				// Should never happen
				log.Debug(
					"skipping invalid tx",
					zap.Error(err),
				)
				return true, false, false, nil
			}
			if b.UnitsConsumed+nextUnits > r.GetMaxBlockUnits() {
				log.Debug(
					"skipping tx: too many units",
					zap.Uint64("block units", b.UnitsConsumed),
					zap.Uint64("tx max units", nextUnits),
				)
				return false /* make simpler */, true, false, nil // could be txs that fit that are smaller
			}

			// Prepare proofs
			if !parent.RootInWindow(ctx, next.Proof.Root) {
				b.vm.Logger().Warn("skipping old tx", zap.Stringer("txID", next.ID()))
				return true, false, false, nil
			}
			nvalues, nnodes := next.Proof.State()
			values := map[ids.ID]map[merkledb.Path]merkledb.Maybe[[]byte]{next.Proof.Root: nvalues}
			nodes := map[ids.ID]map[merkledb.Path]merkledb.Maybe[*merkledb.Node]{next.Proof.Root: nnodes}
			for root, m := range b.values {
				for path, value := range m {
					if _, ok := values[root]; !ok {
						values[root] = make(map[merkledb.Path]merkledb.Maybe[[]byte])
					}
					values[root][path] = value
				}
			}
			for root, m := range b.nodes {
				for path, node := range m {
					if _, ok := nodes[root]; !ok {
						nodes[root] = make(map[merkledb.Path]merkledb.Maybe[*merkledb.Node])
					}
					nodes[root][path] = node
				}
			}
			b.setTemporaryState(
				ctx,
				stateless,
				values,
				nodes,
			)

			// Populate required transaction state and restrict which keys can be used
			//
			// TODO: prefetch state of upcoming txs that we will pull (should make much
			// faster)
			txStart := ts.OpIndex()
			if err := ts.FetchAndSetScope(ctx, next.StateKeys(sm), stateless); err != nil {
				if len(ts.BadKey) > 0 {
					_, ok := b.values[next.Proof.Root][merkledb.NewPath(ts.BadKey)]
					_, pok := values[next.Proof.Root][merkledb.NewPath(ts.BadKey)]
					txValues, _ := next.Proof.State()
					_, tok := txValues[merkledb.NewPath(ts.BadKey)]
					return false, true, false, fmt.Errorf("%w: could not fetch and set scope (key=%x, values=%t, pending values=%t, tx=%t)", err, ts.BadKey, ok, pok, tok)
				}
				return false, true, false, fmt.Errorf("%w: could not fetch and set scope", err)
			}

			// PreExecute next to see if it is fit
			if err := next.PreExecute(fctx, ectx, r, ts, nextTime); err != nil {
				ts.Rollback(ctx, txStart)
				cont, restore, removeAcct := HandlePreExecute(err)
				return cont, restore, removeAcct, nil
			}

			// Verify warp message, if it exists
			//
			// We don't drop invalid warp messages because we must collect fees for
			// the work the sender made us do (otherwise this would be a DoS).
			//
			// We wait as long as possible to verify the signature to ensure we don't
			// spend unnecessary time on an invalid tx.
			var warpErr error
			if next.WarpMessage != nil {
				num, denom, err := preVerifyWarpMessage(next.WarpMessage, vm.ChainID(), r)
				if err == nil {
					warpErr = next.WarpMessage.Signature.Verify(
						ctx, &next.WarpMessage.UnsignedMessage,
						vdrState, blockContext.PChainHeight, num, denom,
					)
				} else {
					warpErr = err
				}
				if warpErr != nil {
					log.Warn(
						"warp verification failed",
						zap.Stringer("txID", next.ID()),
						zap.Error(warpErr),
					)
				}
			}

			// If execution works, keep moving forward with new state
			result, err := next.Execute(
				fctx,
				ectx,
				r,
				sm,
				ts,
				nextTime,
				next.WarpMessage != nil && warpErr == nil,
			)
			if err != nil {
				// This error should only be raised by the handler, not the
				// implementation itself
				log.Warn("unexpected post-execution error", zap.Error(err))
				return false, false, false, err
			}

			// Update block with new transaction
			b.Txs = append(b.Txs, next)
			b.UnitsConsumed += result.Units
			surplusFee += (next.Base.UnitPrice - b.UnitPrice) * result.Units
			results = append(results, result)
			if next.WarpMessage != nil {
				if warpErr == nil {
					// Add a bit if the warp message was verified
					b.WarpResults.Add(uint(warpCount))
				}
				warpCount++
			}
			b.values = values
			b.nodes = nodes
			return len(b.Txs) < r.GetMaxBlockTxs(), false, false, nil
		},
	)
	span.SetAttributes(
		attribute.Int("attempted", txsAttempted),
		attribute.Int("added", len(b.Txs)),
	)
	if mempoolErr != nil {
		b.vm.Mempool().Add(ctx, b.Txs)
		return nil, fmt.Errorf("%w: error while processing tx", mempoolErr)
	}

	// Perform basic validity checks to make sure the block is well-formatted
	if len(b.Txs) == 0 {
		return nil, ErrNoTxs
	}
	requiredSurplus := b.UnitPrice * b.BlockCost
	if surplusFee < requiredSurplus {
		// This is a very common result during block building
		b.vm.Mempool().Add(ctx, b.Txs)
		return nil, fmt.Errorf(
			"%w: required=%d found=%d",
			ErrInsufficientSurplus,
			requiredSurplus,
			surplusFee,
		)
	}
	b.SurplusFee = surplusFee

	// Get root from underlying state changes after writing all changed keys
	if err := ts.WriteChanges(ctx, stateless, vm.Tracer()); err != nil {
		return nil, err
	}
	// TODO: only do if state is not nil
	if err := ts.WriteChanges(ctx, state, vm.Tracer()); err != nil {
		return nil, err
	}

	// Store height in state to prevent duplicate roots
	if err := stateless.Insert(ctx, sm.HeightKey(), binary.BigEndian.AppendUint64(nil, b.Hght)); err != nil {
		return nil, err
	}
	if err := state.Insert(ctx, sm.HeightKey(), binary.BigEndian.AppendUint64(nil, b.Hght)); err != nil {
		return nil, err
	}

	// Compute state root after all data has been written to trie
	root, err := stateless.GetMerkleRoot(ctx)
	if err != nil {
		return nil, err
	}
	sroot, err := state.GetMerkleRoot(ctx)
	if err != nil {
		return nil, err
	}
	if root != sroot {
		return nil, errors.New("state mismatch")
	}
	b.StateRoot = root

	// Compute block hash and marshaled representation
	if err := b.initializeBuilt(ctx, state, stateless, results); err != nil {
		return nil, err
	}
	log.Info(
		"built block",
		zap.Uint64("hght", b.Hght),
		zap.Int("attempted", txsAttempted),
		zap.Int("added", len(b.Txs)),
		zap.Int("mempool size", b.vm.Mempool().Len(ctx)),
		zap.Duration("mempool lock wait", lockWait),
		zap.Bool("context", blockContext != nil),
		zap.Int("state changes", ts.PendingChanges()),
		zap.Int("state operations", ts.OpIndex()),
	)
	return b, nil
}

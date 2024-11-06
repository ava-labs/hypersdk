// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/internal/emap"
	internalfees "github.com/ava-labs/hypersdk/internal/fees"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
	"go.uber.org/zap"
)

type ExecutionConfig struct {
	TargetBuildDuration       time.Duration `json:"targetBuildDuration"`
	TransactionExecutionCores int           `json:"transactionExecutionCores"`
	StateFetchConcurrency     int           `json:"stateFetchConcurrency"`
}

func NewDefaultExecutionConfig() ExecutionConfig {
	return ExecutionConfig{
		TargetBuildDuration:       100 * time.Millisecond,
		TransactionExecutionCores: 1,
		StateFetchConcurrency:     1,
	}
}

type Chain struct {
	tracer                  trace.Tracer
	config                  ExecutionConfig
	metrics                 *chainMetrics
	mempool                 Mempool
	log                     logging.Logger
	parser                  Parser
	authVerificationWorkers workers.Workers
	ruleFactory             RuleFactory
	metadataManager         MetadataManager
	balanceHandler          BalanceHandler
	authVM                  AuthVM

	lock       sync.Mutex
	chainIndex ChainIndex
	seen       *emap.EMap[*Transaction]
}

type ChainIndex interface {
	GetExecutionBlock(ctx context.Context, blkID ids.ID) (*ExecutionBlock, error)
	LastAcceptedExecutionBlock() *ExecutionBlock
}

type ChainBackend interface {
	Tracer() trace.Tracer
	Metrics() metrics.MultiGatherer
	Mempool() Mempool
	Logger() logging.Logger
	Parser
	RuleFactory() RuleFactory
	MetadataManager() MetadataManager
	BalanceHandler() BalanceHandler
	AuthVerifiers() workers.Workers
	GetAuthBatchVerifier(authTypeID uint8, cores int, count int) (AuthBatchVerifier, bool)
	ChainIndex
}

func NewChain(
	backend ChainBackend,
	config ExecutionConfig,
) (*Chain, error) {
	registry, metrics, err := newMetrics()
	if err != nil {
		return nil, err
	}
	if err := backend.Metrics().Register("chain", registry); err != nil {
		return nil, err
	}
	return &Chain{
		tracer:                  backend.Tracer(),
		config:                  config,
		metrics:                 metrics,
		mempool:                 backend.Mempool(),
		log:                     backend.Logger(),
		authVerificationWorkers: backend.AuthVerifiers(),
		parser:                  backend,
		ruleFactory:             backend.RuleFactory(),
		metadataManager:         backend.MetadataManager(),
		balanceHandler:          backend.BalanceHandler(),
		authVM:                  backend,
		chainIndex:              backend,
		seen:                    emap.NewEMap[*Transaction](),
	}, nil
}

func (c *Chain) AcceptBlock(ctx context.Context, blk *ExecutionBlock) error {
	_, span := c.tracer.Start(ctx, "chain.AcceptBlock")
	defer span.End()

	// Grab the lock before modifiying seen
	c.lock.Lock()
	defer c.lock.Unlock()

	blkTime := blk.Tmstmp
	evicted := c.seen.SetMin(blkTime)
	c.log.Debug("txs evicted from seen", zap.Int("len", len(evicted)))
	c.seen.Add(blk.Txs)
	return nil
}

func (c *Chain) PreExecute(
	ctx context.Context,
	parentBlk *ExecutionBlock,
	view state.View,
	tx *Transaction,
	verifyAuth bool,
) error {
	feeRaw, err := view.GetValue(ctx, FeeKey(c.metadataManager.FeePrefix()))
	if err != nil {
		return err
	}
	feeManager := internalfees.NewManager(feeRaw)
	now := time.Now().UnixMilli()
	r := c.ruleFactory.GetRules(now)
	nextFeeManager, err := feeManager.ComputeNext(now, r)
	if err != nil {
		return err
	}

	// Find repeats
	repeatErrs, err := c.IsRepeat(ctx, parentBlk, now, []*Transaction{tx})
	if err != nil {
		return err
	}
	if repeatErrs.BitLen() > 0 {
		return ErrDuplicateTx
	}

	// Ensure state keys are valid
	_, err = tx.StateKeys(c.balanceHandler)
	if err != nil {
		return err
	}

	// Verify auth if not already verified by caller
	if verifyAuth {
		if err := tx.VerifyAuth(ctx); err != nil {
			return err
		}
	}

	// PreExecute does not make any changes to state
	//
	// This may fail if the state we are utilizing is invalidated (if a trie
	// view from a different branch is committed underneath it). We prefer this
	// instead of putting a lock around all commits.
	//
	// Note, [PreExecute] ensures that the pending transaction does not have
	// an expiry time further ahead than [ValidityWindow]. This ensures anything
	// added to the [Mempool] is immediately executable.
	if err := tx.PreExecute(ctx, nextFeeManager, c.balanceHandler, r, view, now); err != nil {
		return err
	}
	return nil
}

func (c *Chain) IsRepeat(
	ctx context.Context,
	parentBlk *ExecutionBlock,
	timestamp int64,
	txs []*Transaction,
) (set.Bits, error) {
	_, span := c.tracer.Start(ctx, "chain.IsRepeat")
	defer span.End()

	rules := c.ruleFactory.GetRules(timestamp)

	oldestAllowed := timestamp - rules.GetValidityWindow()
	if oldestAllowed < 0 {
		// Can occur if verifying genesis
		oldestAllowed = 0
	}

	return c.isRepeat(ctx, parentBlk, timestamp, txs, set.NewBits(), false)
}

func (c *Chain) isRepeat(
	ctx context.Context,
	ancestorBlk *ExecutionBlock,
	oldestAllowed int64,
	txs []*Transaction,
	marker set.Bits,
	stop bool,
) (set.Bits, error) {
	_, span := c.tracer.Start(ctx, "chain.IsRepeat")
	defer span.End()

	c.lock.Lock()
	defer c.lock.Unlock()

	lastAcceptedBlk := c.chainIndex.LastAcceptedExecutionBlock()

	var err error
	for {
		if ancestorBlk.Tmstmp < oldestAllowed {
			return marker, nil
		}

		if ancestorBlk.Hght <= lastAcceptedBlk.Hght || ancestorBlk.Hght == 0 {
			return c.seen.Contains(txs, marker, stop), nil
		}

		for i, tx := range txs {
			if marker.Contains(i) {
				continue
			}
			if ancestorBlk.txsSet.Contains(tx.ID()) {
				marker.Add(i)
				if stop {
					return marker, nil
				}
			}
		}

		ancestorBlk, err = c.chainIndex.GetExecutionBlock(ctx, ancestorBlk.Prnt)
		if err != nil {
			return marker, err
		}
	}
}

func (c *Chain) verifyExpiryReplayProtection(
	ctx context.Context,
	blk *ExecutionBlock,
	rules Rules,
) error {
	lastAcceptedBlk := c.chainIndex.LastAcceptedExecutionBlock()
	if blk.Hght <= lastAcceptedBlk.Hght {
		return nil
	}
	parent, err := c.chainIndex.GetExecutionBlock(ctx, blk.Prnt)
	if err != nil {
		return err
	}

	oldestAllowed := blk.Tmstmp - rules.GetValidityWindow()
	if oldestAllowed < 0 {
		// Can occur if verifying genesis
		oldestAllowed = 0
	}
	dup, err := c.isRepeat(ctx, parent, oldestAllowed, blk.Txs, set.NewBits(), true)
	if err != nil {
		return err
	}
	if dup.Len() > 0 {
		return fmt.Errorf("%w: duplicate in ancestry", ErrDuplicateTx)
	}
	return nil
}

func (c *Chain) ParseBlock(ctx context.Context, b []byte) (*ExecutionBlock, error) {
	_, span := c.tracer.Start(ctx, "chain.ParseBlock")
	defer span.End()

	blk, err := UnmarshalBlock(b, c.parser)
	if err != nil {
		return nil, err
	}

	lastAcceptedBlock := c.chainIndex.LastAcceptedExecutionBlock()
	return NewExecutionBlock(ctx, blk, c, blk.Hght > lastAcceptedBlock.Hght)
}

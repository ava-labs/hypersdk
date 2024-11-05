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
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/utils"
	"go.uber.org/zap"
)

type ExecutionBlock struct {
	*StatelessBlock

	id     ids.ID
	bytes  []byte
	sigJob workers.Job
	txsSet set.Set[ids.ID]
}

type ExecutionConfig struct {
	TargetBuildDuration       time.Duration `json:"targetBuildDuration"`
	TransactionExecutionCores int           `json:"transactionExecutionCores"`
	StateFetchConcurrency     int           `json:"stateFetchConcurrency"`
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
	seen       emap.EMap[*Transaction]
}

type ChainIndex interface {
	GetBlock(ctx context.Context, blkID ids.ID) (*ExecutionBlock, error)
	LastAcceptedBlock(ctx context.Context) (*ExecutionBlock, error)
}

type ChainBackend interface {
	Tracer() trace.Tracer
	Metrics() metrics.MultiGatherer
	Mempool() Mempool
	Logger() logging.Logger
	Parser() Parser
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
	chainIndex ChainIndex,
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
		parser:                  backend.Parser(),
		ruleFactory:             backend.RuleFactory(),
		metadataManager:         backend.MetadataManager(),
		balanceHandler:          backend.BalanceHandler(),
		authVM:                  backend,
		chainIndex:              backend,
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

	lastAcceptedBlk, err := c.chainIndex.LastAcceptedBlock(ctx)
	if err != nil {
		return marker, err
	}

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

		ancestorBlk, err = c.chainIndex.GetBlock(ctx, ancestorBlk.Prnt)
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
	lastAcceptedBlk, err := c.chainIndex.LastAcceptedBlock(ctx)
	if err != nil {
		return err
	}
	if blk.Hght <= lastAcceptedBlk.Hght {
		return nil
	}
	parent, err := c.chainIndex.GetBlock(ctx, blk.Prnt)
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

	// Setup signature verification job
	_, sigVerifySpan := c.tracer.Start(ctx, "ParseBlock.verifySignatures") //nolint:spancheck
	sigJob, err := c.authVerificationWorkers.NewJob(len(blk.Txs))
	if err != nil {
		return nil, err //nolint:spancheck
	}
	batchVerifier := NewAuthBatch(c.authVM, sigJob, blk.AuthCounts)

	// Make sure to always call [Done], otherwise we will block all future [Workers]
	defer func() {
		// BatchVerifier is given the responsibility to call [b.sigJob.Done()] because it may add things
		// to the work queue async and that may not have completed by this point.
		go batchVerifier.Done(func() { sigVerifySpan.End() })
	}()

	// Confirm no transaction duplicates and setup
	// signature verification
	txsSet := set.NewSet[ids.ID](len(blk.Txs))
	for _, tx := range blk.Txs {
		// Ensure there are no duplicate transactions
		if txsSet.Contains(tx.ID()) {
			return nil, ErrDuplicateTx
		}
		txsSet.Add(tx.ID())

		// Verify signature async
		unsignedTxBytes, err := tx.UnsignedBytes()
		if err != nil {
			return nil, err
		}
		batchVerifier.Add(unsignedTxBytes, tx.Auth)
	}
	return &ExecutionBlock{
		StatelessBlock: blk,
		sigJob:         sigJob,
		txsSet:         txsSet,
		id:             utils.ToID(b),
		bytes:          b,
	}, nil
}

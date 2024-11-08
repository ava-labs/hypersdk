// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"

	internalfees "github.com/ava-labs/hypersdk/internal/fees"
)

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
	GetValidityWindow() *TimeValidityWindow
}

type Chain struct {
	tracer                  trace.Tracer
	config                  Config
	metrics                 *chainMetrics
	mempool                 Mempool
	log                     logging.Logger
	parser                  Parser
	authVerificationWorkers workers.Workers
	ruleFactory             RuleFactory
	metadataManager         MetadataManager
	balanceHandler          BalanceHandler
	authVM                  AuthVM
	validityWindow          *TimeValidityWindow
}

func NewChain(
	backend ChainBackend,
	config Config,
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
		validityWindow:          backend.GetValidityWindow(),
	}, nil
}

func (c *Chain) AcceptBlock(ctx context.Context, blk *ExecutionBlock) error {
	_, span := c.tracer.Start(ctx, "chain.AcceptBlock")
	defer span.End()

	c.metrics.txsAccepted.Add(float64(len(blk.Txs)))

	c.validityWindow.Accept(blk)

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
	oldestAllowed := now - r.GetValidityWindow()
	if oldestAllowed < 0 {
		oldestAllowed = 0
	}
	repeatErrs, err := c.validityWindow.IsRepeat(ctx, parentBlk, []*Transaction{tx}, oldestAllowed)
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

func (c *Chain) ParseBlock(ctx context.Context, b []byte) (*ExecutionBlock, error) {
	_, span := c.tracer.Start(ctx, "chain.ParseBlock")
	defer span.End()

	blk, err := UnmarshalBlock(b, c.parser)
	if err != nil {
		return nil, err
	}
	return NewExecutionBlock(blk), nil
}

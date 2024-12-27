// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
)

// create a type alias for the concrete TimeWindowWindow type.
type ValidityWindow = *validitywindow.TimeValidityWindow[*Transaction]

type Chain struct {
	builder     *Builder
	processor   *Processor
	preExecutor *PreExecutor
	blockParser *BlockParser
}

func NewChain(
	tracer trace.Tracer,
	registerer *prometheus.Registry,
	parser Parser,
	mempool Mempool,
	logger logging.Logger,
	ruleFactory RuleFactory,
	metadataManager MetadataManager,
	balanceHandler BalanceHandler,
	authVerifiers workers.Workers,
	authVM AuthVM,
	validityWindow ValidityWindow,
	config Config,
) (*Chain, error) {
	metrics, err := newMetrics(registerer)
	if err != nil {
		return nil, err
	}
	return &Chain{
		builder: NewBuilder(
			tracer,
			ruleFactory,
			logger,
			metadataManager,
			balanceHandler,
			mempool,
			validityWindow,
			metrics,
			config,
		),
		processor: NewProcessor(
			tracer,
			logger,
			ruleFactory,
			authVerifiers,
			authVM,
			metadataManager,
			balanceHandler,
			validityWindow,
			metrics,
			config,
		),
		preExecutor: NewPreExecutor(
			ruleFactory,
			validityWindow,
			metadataManager,
			balanceHandler,
		),
		blockParser: NewBlockParser(tracer, parser),
	}, nil
}

func (c *Chain) BuildBlock(ctx context.Context, parentOutputBlock *OutputBlock) (*ExecutionBlock, *OutputBlock, error) {
	return c.builder.BuildBlock(ctx, parentOutputBlock)
}

func (c *Chain) Execute(
	ctx context.Context,
	parentBlock *OutputBlock,
	b *ExecutionBlock,
) (*OutputBlock, error) {
	return c.processor.Execute(ctx, parentBlock, b)
}

func (c *Chain) AsyncVerify(
	ctx context.Context,
	b *ExecutionBlock,
) error {
	return c.processor.AsyncVerify(ctx, b)
}

func (c *Chain) PreExecute(
	ctx context.Context,
	parentBlk *ExecutionBlock,
	view state.View,
	tx *Transaction,
) error {
	return c.preExecutor.PreExecute(ctx, parentBlk, view, tx)
}

func (c *Chain) ParseBlock(ctx context.Context, bytes []byte) (*ExecutionBlock, error) {
	return c.blockParser.ParseBlock(ctx, bytes)
}

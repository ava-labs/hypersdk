// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"

	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
)

type Chain struct {
	builder     *Builder
	processor   *Processor
	preExecutor *PreExecutor
	blockParser *BlockParser
	accepter    *Accepter
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
	authEngines AuthEngines,
	validityWindow ValidityWindow,
	config Config,
) (*Chain, error) {
	metrics, err := NewMetrics(registerer)
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
			authEngines,
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
		accepter:    NewAccepter(tracer, validityWindow, metrics),
	}, nil
}

func (c *Chain) BuildBlock(ctx context.Context, pChainCtx *block.Context, parentOutputBlock *OutputBlock) (*ExecutionBlock, *OutputBlock, error) {
	return c.builder.BuildBlock(ctx, pChainCtx, parentOutputBlock)
}

func (c *Chain) Execute(
	ctx context.Context,
	parentView merkledb.View,
	b *ExecutionBlock,
	isNormalOp bool,
) (*OutputBlock, error) {
	return c.processor.Execute(ctx, parentView, b, isNormalOp)
}

func (c *Chain) PreExecute(
	ctx context.Context,
	parentBlk *ExecutionBlock,
	im state.Immutable,
	tx *Transaction,
) error {
	return c.preExecutor.PreExecute(ctx, parentBlk, im, tx)
}

func (c *Chain) ParseBlock(ctx context.Context, bytes []byte) (*ExecutionBlock, error) {
	return c.blockParser.ParseBlock(ctx, bytes)
}

func (c *Chain) AcceptBlock(ctx context.Context, block *OutputBlock) error {
	return c.accepter.AcceptBlock(ctx, block)
}

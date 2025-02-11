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

type Chain[T Action[T], A Auth[A]] struct {
	builder     *Builder[T, A]
	processor   *Processor[T, A]
	preExecutor *PreExecutor[T, A]
	blockParser *BlockParser[T, A]
	accepter    *Accepter[T, A]
}

func NewChain[T Action[T], A Auth[A]](
	tracer trace.Tracer,
	registerer *prometheus.Registry,
	mempool Mempool[T, A],
	logger logging.Logger,
	ruleFactory RuleFactory,
	metadataManager MetadataManager,
	balanceHandler BalanceHandler,
	authVerifiers workers.Workers,
	authVM AuthVM,
	validityWindow ValidityWindow[T, A],
	config Config,
) (*Chain[T, A], error) {
	metrics, err := newMetrics(registerer)
	if err != nil {
		return nil, err
	}
	return &Chain[T, A]{
		builder: NewBuilder[T, A](
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
		processor: NewProcessor[T, A](
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
		preExecutor: NewPreExecutor[T, A](
			ruleFactory,
			validityWindow,
			metadataManager,
			balanceHandler,
		),
		blockParser: NewBlockParser[T, A](tracer),
		accepter:    NewAccepter[T, A](tracer, validityWindow, metrics),
	}, nil
}

func (c *Chain[T, A]) BuildBlock(ctx context.Context, pChainCtx *block.Context, parentOutputBlock *OutputBlock[T, A]) (*ExecutionBlock[T, A], *OutputBlock[T, A], error) {
	return c.builder.BuildBlock(ctx, pChainCtx, parentOutputBlock)
}

func (c *Chain[T, A]) Execute(
	ctx context.Context,
	parentView merkledb.View,
	b *ExecutionBlock[T, A],
	isNormalOp bool,
) (*OutputBlock[T, A], error) {
	return c.processor.Execute(ctx, parentView, b, isNormalOp)
}

func (c *Chain[T, A]) PreExecute(
	ctx context.Context,
	parentBlk *ExecutionBlock[T, A],
	im state.Immutable,
	tx *Transaction[T, A],
) error {
	return c.preExecutor.PreExecute(ctx, parentBlk, im, tx)
}

func (c *Chain[T, A]) ParseBlock(ctx context.Context, bytes []byte) (*ExecutionBlock[T, A], error) {
	return c.blockParser.ParseBlock(ctx, bytes)
}

func (c *Chain[T, A]) AcceptBlock(ctx context.Context, block *OutputBlock[T, A]) error {
	return c.accepter.AcceptBlock(ctx, block)
}

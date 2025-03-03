// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossiper

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/consts"
)

var _ Gossiper = (*Manual[Tx])(nil)

type Manual[T Tx] struct {
	log                  logging.Logger
	metrics              *metrics
	mempool              Mempool[T]
	serializer           Serializer[T]
	submitter            Submitter[T]
	targetGossipDuration time.Duration
	client               *p2p.Client
	doneGossip           chan struct{}
}

func NewManual[T Tx](
	log logging.Logger,
	registerer prometheus.Registerer,
	mempool Mempool[T],
	serializer Serializer[T],
	submitter Submitter[T],
	targetGossipDuration time.Duration,
) (*Manual[T], error) {
	metrics, err := newMetrics(registerer)
	if err != nil {
		return nil, err
	}

	return &Manual[T]{
		log:                  log,
		metrics:              metrics,
		mempool:              mempool,
		serializer:           serializer,
		submitter:            submitter,
		targetGossipDuration: targetGossipDuration,
		doneGossip:           make(chan struct{}),
	}, nil
}

func (g *Manual[T]) Start(client *p2p.Client) {
	g.client = client

	// Only respond to explicitly triggered gossip
	close(g.doneGossip)
}

func (g *Manual[T]) Force(ctx context.Context) error {
	// Gossip highest paying txs
	var (
		txs  = []T{}
		size = 0
		now  = time.Now().UnixMilli()
	)
	mempoolErr := g.mempool.Top(
		ctx,
		g.targetGossipDuration,
		func(_ context.Context, next T) (cont bool, rest bool, err error) {
			// Remove txs that are expired
			if next.GetExpiry() < now {
				return true, false, nil
			}

			// Gossip up to [consts.NetworkSizeLimit]
			txSize := next.Size()
			if txSize+size > consts.NetworkSizeLimit {
				return false, true, nil
			}
			txs = append(txs, next)
			size += txSize
			return true, true, nil
		},
	)
	if mempoolErr != nil {
		return mempoolErr
	}
	if len(txs) == 0 {
		return nil
	}
	txBatchBytes := g.serializer.Marshal(txs)
	if err := g.client.AppGossip(ctx, common.SendConfig{Validators: 10}, txBatchBytes); err != nil {
		g.log.Warn(
			"GossipTxs failed",
			zap.Error(err),
		)
		return err
	}
	g.log.Debug("gossiped txs", zap.Int("count", len(txs)))
	return nil
}

func (g *Manual[T]) HandleAppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	txs, err := g.serializer.Unmarshal(msg)
	if err != nil {
		g.log.Warn(
			"AppGossip provided invalid txs",
			zap.Stringer("peerID", nodeID),
			zap.Error(err),
		)
		return nil
	}
	g.metrics.txsReceived.Add(float64(len(txs)))

	start := time.Now()
	numErrs := 0
	for _, err := range g.submitter.Submit(ctx, txs) {
		if err != nil {
			numErrs++
		}
	}
	g.log.Info(
		"tx gossip received",
		zap.Int("txs", len(txs)),
		zap.Int("numFailedSubmit", numErrs),
		zap.Stringer("nodeID", nodeID),
		zap.Duration("t", time.Since(start)),
	)
	return nil
}

func (g *Manual[T]) Done() {
	<-g.doneGossip
}

// Queue is a no-op in [Manual].
func (*Manual[T]) Queue(context.Context) {}

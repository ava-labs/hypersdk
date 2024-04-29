// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossiper

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
)

var _ Gossiper = (*Manual)(nil)

type Manual struct {
	vm         VM
	appSender  common.AppSender
	doneGossip chan struct{}
}

func NewManual(vm VM) *Manual {
	return &Manual{
		vm:         vm,
		doneGossip: make(chan struct{}),
	}
}

func (g *Manual) Run(appSender common.AppSender) {
	g.appSender = appSender

	// Only respond to explicitly triggered gossip
	close(g.doneGossip)
}

func (g *Manual) Force(ctx context.Context) error {
	// Gossip highest paying txs
	var (
		txs  = []*chain.Transaction{}
		size = 0
		now  = time.Now().UnixMilli()
	)
	mempoolErr := g.vm.Mempool().Top(
		ctx,
		g.vm.GetTargetGossipDuration(),
		func(_ context.Context, next *chain.Transaction) (cont bool, rest bool, err error) {
			// Remove txs that are expired
			if next.Base.Timestamp < now {
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
	b, err := chain.MarshalTxs(txs)
	if err != nil {
		return err
	}
	if err := g.appSender.SendAppGossip(ctx, b); err != nil {
		g.vm.Logger().Warn(
			"GossipTxs failed",
			zap.Error(err),
		)
		return err
	}
	g.vm.Logger().Debug("gossiped txs", zap.Int("count", len(txs)))
	return nil
}

func (g *Manual) HandleAppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	actionRegistry, authRegistry := g.vm.Registry()
	_, txs, err := chain.UnmarshalTxs(msg, initialCapacity, actionRegistry, authRegistry)
	if err != nil {
		g.vm.Logger().Warn(
			"AppGossip provided invalid txs",
			zap.Stringer("peerID", nodeID),
			zap.Error(err),
		)
		return nil
	}
	g.vm.RecordTxsReceived(len(txs))

	start := time.Now()
	for _, err := range g.vm.Submit(ctx, true, txs) {
		if err == nil {
			continue
		}
		g.vm.Logger().Warn(
			"AppGossip failed to submit txs",
			zap.Stringer("peerID", nodeID),
			zap.Error(err),
		)
	}
	g.vm.Logger().Info(
		"tx gossip received",
		zap.Int("txs", len(txs)),
		zap.Stringer("nodeID", nodeID),
		zap.Duration("t", time.Since(start)),
	)
	return nil
}

func (*Manual) BlockVerified(int64) {}

func (g *Manual) Done() {
	<-g.doneGossip
}

// Queue is a no-op in [Manual].
func (*Manual) Queue(context.Context) {}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossiper

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/AnomalyFi/hypersdk/chain"
	"go.uber.org/zap"
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

func (g *Manual) TriggerGossip(ctx context.Context) error {
	// Gossip highest paying txs
	txs := []*chain.Transaction{}
	totalUnits := uint64(0)
	now := time.Now().Unix()
	r := g.vm.Rules(now)
	mempoolErr := g.vm.Mempool().Build(
		ctx,
		func(ictx context.Context, next *chain.Transaction) (cont bool, restore bool, removeAcct bool, err error) {
			// Remove txs that are expired
			if next.Base.Timestamp < now {
				return true, false, false, nil
			}

			// Gossip up to a block of content
			units, err := next.MaxUnits(r)
			if err != nil {
				// Should never happen
				return true, false, false, nil
			}
			if units+totalUnits > r.GetMaxBlockUnits() {
				// Attempt to mirror the function of building a block without execution
				return false, true, false, nil
			}
			txs = append(txs, next)
			totalUnits += units
			return len(txs) < r.GetMaxBlockTxs(), true, false, nil
		},
	)
	if mempoolErr != nil {
		return mempoolErr
	}
	if len(txs) == 0 {
		return nil
	}
	actionRegistry, authRegistry := g.vm.Registry()
	b, err := chain.MarshalTxs(txs, actionRegistry, authRegistry)
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
	r := g.vm.Rules(time.Now().Unix())
	actionRegistry, authRegistry := g.vm.Registry()
	txs, err := chain.UnmarshalTxs(msg, r.GetMaxBlockTxs(), actionRegistry, authRegistry)
	if err != nil {
		g.vm.Logger().Warn(
			"AppGossip provided invalid txs",
			zap.Stringer("peerID", nodeID),
			zap.Error(err),
		)
		return nil
	}

	g.vm.Logger().Info("AppGossip transactions are being submitted", zap.Int("txs", len(txs)))
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
	return nil
}

func (*Manual) BlockVerified(int64) {}

func (g *Manual) Done() {
	<-g.doneGossip
}

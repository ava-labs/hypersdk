// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossiper

import (
	"context"
	"errors"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/proposervm/proposer"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/workers"
	"go.uber.org/zap"
)

var _ Gossiper = (*Proposer)(nil)

var proposerWindow = proposer.MaxDelay.Milliseconds()

type Proposer struct {
	vm         VM
	cfg        *ProposerConfig
	appSender  common.AppSender
	doneGossip chan struct{}

	lastVerified int64
}

type ProposerConfig struct {
	GossipProposerDiff  int
	GossipProposerDepth int
	GossipInterval      time.Duration
	GossipMinLife       int64 // ms
	GossipMinSize       int
	GossipMaxDelay      int64 // ms
	GossipMaxSize       int
	NoGossipBuilderDiff int
	VerifyTimeout       int64 // ms
}

func DefaultProposerConfig() *ProposerConfig {
	return &ProposerConfig{
		GossipProposerDiff:  3,
		GossipProposerDepth: 1,
		GossipMinLife:       5 * 1000,
		GossipMinSize:       32 * units.KiB, // TODO: need to add size tracking to mempool
		GossipMaxDelay:      50,             // wait for ms if below min size
		GossipMaxSize:       consts.NetworkSizeLimit,
		NoGossipBuilderDiff: 5,
		VerifyTimeout:       proposerWindow / 2,
	}
}

func NewProposer(vm VM, cfg *ProposerConfig) *Proposer {
	return &Proposer{
		vm:           vm,
		cfg:          cfg,
		doneGossip:   make(chan struct{}),
		lastVerified: -1,
	}
}

func (g *Proposer) sendTxs(ctx context.Context, txs []*chain.Transaction) error {
	ctx, span := g.vm.Tracer().Start(ctx, "Gossiper.sendTxs")
	defer span.End()

	// Marshal gossip
	b, err := chain.MarshalTxs(txs)
	if err != nil {
		return err
	}

	// Select next set of proposers and send gossip to them
	proposers, err := g.vm.Proposers(
		ctx,
		g.cfg.GossipProposerDiff,
		g.cfg.GossipProposerDepth,
	)
	if err != nil || proposers.Len() == 0 {
		g.vm.Logger().Warn(
			"unable to find any proposers, falling back to all-to-all gossip",
			zap.Error(err),
		)
		return g.appSender.SendAppGossip(ctx, b)
	}
	recipients := set.NewSet[ids.NodeID](len(proposers))
	for proposer := range proposers {
		// Don't gossip to self
		if proposer == g.vm.NodeID() {
			continue
		}
		recipients.Add(proposer)
	}
	return g.appSender.SendAppGossipSpecific(ctx, recipients, b)
}

func (g *Proposer) Force(ctx context.Context) error {
	ctx, span := g.vm.Tracer().Start(ctx, "Gossiper.GossipTxs")
	defer span.End()

	// Gossip highest paying txs
	var (
		txs   = []*chain.Transaction{}
		size  = 0
		start = time.Now()
		now   = start.UnixMilli()
	)
	mempoolErr := g.vm.Mempool().Top(
		ctx,
		g.vm.GetTargetGossipDuration(),
		func(ictx context.Context, next *chain.Transaction) (cont bool, rest bool, err error) {
			// Remove txs that are expired
			if next.Base.Timestamp < now {
				return true, false, nil
			}

			// Don't gossip txs that are about to expire
			life := next.Base.Timestamp - now
			if life < g.cfg.GossipMinLife {
				return true, true, nil
			}

			// Gossip up to [GossipMaxSize]
			txSize := next.Size()
			if txSize+size > g.cfg.GossipMaxSize {
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
	g.vm.Logger().Info("gossiping transactions", zap.Int("txs", len(txs)), zap.Duration("t", time.Since(start)))
	g.vm.RecordTxsGossiped(len(txs))
	return g.sendTxs(ctx, txs)
}

func (g *Proposer) HandleAppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	actionRegistry, authRegistry := g.vm.Registry()
	authCounts, txs, err := chain.UnmarshalTxs(msg, initialCapacity, actionRegistry, authRegistry)
	if err != nil {
		g.vm.Logger().Warn(
			"received invalid txs",
			zap.Stringer("peerID", nodeID),
			zap.Error(err),
		)
		return nil
	}
	g.vm.RecordTxsReceived(len(txs))

	// Add incoming transactions to our caches to prevent useless gossip and perform
	// batch signature verification.
	//
	// We rely on AppGossipConcurrency to regulate concurrency here, so we don't create
	// a separate pool of workers for this verification.
	job, err := workers.NewSerial().NewJob(len(txs))
	if err != nil {
		g.vm.Logger().Warn(
			"unable to spawn new worker",
			zap.Stringer("peerID", nodeID),
			zap.Error(err),
		)
		return nil
	}
	batchVerifier := chain.NewAuthBatch(g.vm, job, authCounts)
	for _, tx := range txs {
		// Verify signature async
		txDigest, err := tx.Digest()
		if err != nil {
			g.vm.Logger().Warn(
				"unable to compute tx digest",
				zap.Stringer("peerID", nodeID),
				zap.Error(err),
			)
			batchVerifier.Done(nil)
			return nil
		}
		batchVerifier.Add(txDigest, tx.Auth)
	}
	batchVerifier.Done(nil)

	// Wait for signature verification to finish
	if err := job.Wait(); err != nil {
		g.vm.Logger().Warn(
			"received invalid gossip",
			zap.Stringer("peerID", nodeID),
			zap.Error(err),
		)
		return nil
	}

	// Mark incoming gossip as held by [nodeID], if it is a validator
	isValidator, err := g.vm.IsValidator(ctx, nodeID)
	if err != nil {
		g.vm.Logger().Warn(
			"unable to determine if nodeID is validator",
			zap.Stringer("peerID", nodeID),
			zap.Error(err),
		)
	}

	// Submit incoming gossip to mempool
	start := time.Now()
	for _, err := range g.vm.Submit(ctx, false, txs) {
		if err == nil || errors.Is(err, chain.ErrDuplicateTx) {
			continue
		}
		g.vm.Logger().Debug(
			"failed to submit gossiped txs",
			zap.Stringer("nodeID", nodeID),
			zap.Bool("validator", isValidator),
			zap.Error(err),
		)
	}
	g.vm.Logger().Info(
		"tx gossip received",
		zap.Int("txs", len(txs)),
		zap.Stringer("nodeID", nodeID),
		zap.Bool("validator", isValidator),
		zap.Duration("t", time.Since(start)),
	)

	// only trace error to prevent VM's being shutdown
	// from "AppGossip" returning an error
	return nil
}

func (g *Proposer) Queue() {
}

// periodically but less aggressively force-regossip the pending
func (g *Proposer) Run(appSender common.AppSender) {
	g.appSender = appSender

	g.vm.Logger().Info("starting gossiper", zap.Duration("interval", g.cfg.GossipInterval))
	defer close(g.doneGossip)

	t := time.NewTicker(g.cfg.GossipInterval)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			tctx := context.Background()

			// Check if we are going to propose if it has been less than
			// [VerifyTimeout] since the last time we verified a block.
			if time.Now().UnixMilli()-g.lastVerified < g.cfg.VerifyTimeout {
				proposers, err := g.vm.Proposers(
					tctx,
					g.cfg.NoGossipBuilderDiff,
					1,
				)
				if err == nil && proposers.Contains(g.vm.NodeID()) {
					g.vm.Logger().Debug("not gossiping because soon to propose")
					continue
				} else if err != nil {
					g.vm.Logger().Warn("unable to determine if will propose soon, gossiping anyways", zap.Error(err))
				}
			}

			// Gossip to proposers who will produce next
			if err := g.Force(tctx); err != nil {
				g.vm.Logger().Warn("gossip txs failed", zap.Error(err))
			}
		case <-g.vm.StopChan():
			g.vm.Logger().Info("stopping gossip loop")
			return
		}
	}
}

func (g *Proposer) BlockVerified(t int64) {
	if t < g.lastVerified {
		return
	}
	g.lastVerified = t
}

func (g *Proposer) Done() {
	<-g.doneGossip
}

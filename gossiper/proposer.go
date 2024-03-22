// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossiper

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/timer"
	"github.com/ava-labs/avalanchego/vms/proposervm/proposer"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/cache"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/workers"
)

var _ Gossiper = (*Proposer)(nil)

type Proposer struct {
	vm         VM
	cfg        *ProposerConfig
	appSender  common.AppSender
	doneGossip chan struct{}

	lastVerified int64

	fl sync.Mutex

	q         chan struct{}
	lastQueue int64
	timer     *timer.Timer
	waiting   atomic.Bool

	// cache is thread-safe
	cache *cache.FIFO[ids.ID, any]
}

type ProposerConfig struct {
	GossipProposerDiff  int
	GossipProposerDepth int
	GossipMinLife       int64 // ms
	GossipMaxSize       int
	GossipMinDelay      int64 // ms
	NoGossipBuilderDiff int
	VerifyTimeout       int64 // ms
	SeenCacheSize       int
}

func DefaultProposerConfig() *ProposerConfig {
	return &ProposerConfig{
		GossipProposerDiff:  4,
		GossipProposerDepth: 1,
		GossipMinLife:       5 * 1000,
		GossipMaxSize:       consts.NetworkSizeLimit,
		GossipMinDelay:      50,
		NoGossipBuilderDiff: 4,
		VerifyTimeout:       proposer.MaxVerifyDelay.Milliseconds(),
		SeenCacheSize:       2_500_000,
	}
}

func NewProposer(vm VM, cfg *ProposerConfig) (*Proposer, error) {
	g := &Proposer{
		vm:         vm,
		cfg:        cfg,
		doneGossip: make(chan struct{}),

		lastVerified: -1,

		q:         make(chan struct{}),
		lastQueue: -1,
	}
	g.timer = timer.NewTimer(g.handleTimerNotify)
	cache, err := cache.NewFIFO[ids.ID, any](cfg.SeenCacheSize)
	if err != nil {
		return nil, err
	}
	g.cache = cache
	return g, nil
}

func (g *Proposer) Force(ctx context.Context) error {
	ctx, span := g.vm.Tracer().Start(ctx, "Gossiper.Force")
	defer span.End()

	g.fl.Lock()
	defer g.fl.Unlock()

	// Gossip newest transactions
	//
	// We remove these transactions from the mempool
	// otherwise we'll just keep sending the same FIFO txs
	// to the network over and over.
	//
	// If we are going to build, we should never be attempting
	// to gossip and we should hold on to the txs we
	// could execute. By gossiping, we are basically saying that
	// it is better if someone else builds with these txs because
	// that increases the probability they'll be accepted
	// before they expire.
	var (
		txs   = []*chain.Transaction{}
		size  = 0
		start = time.Now()
		now   = start.UnixMilli()
	)
	mempoolErr := g.vm.Mempool().Top(
		ctx,
		g.vm.GetTargetGossipDuration(),
		func(_ context.Context, next *chain.Transaction) (cont bool, rest bool, err error) {
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

			// Don't remove anything from mempool
			// that will be dropped (this seems
			// like we sent it then got sent it back?)
			txID := next.ID()
			if _, ok := g.cache.Get(txID); ok {
				return true, true, nil
			}
			g.cache.Put(txID, nil)

			txs = append(txs, next)
			size += txSize
			return true, false, nil
		},
	)
	if mempoolErr != nil {
		return mempoolErr
	}
	if len(txs) == 0 {
		g.vm.Logger().Debug("no transactions to gossip")
		return nil
	}
	g.vm.Logger().Debug("gossiping transactions", zap.Int("txs", len(txs)), zap.Duration("t", time.Since(start)))
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
	var seen int
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

		// Add incoming txs to the cache to make
		// sure we never gossip anything we receive (someone
		// else will)
		if g.cache.Put(tx.ID(), nil) {
			seen++
		}
	}
	batchVerifier.Done(nil)
	g.vm.RecordSeenTxsReceived(seen)

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
	g.vm.Logger().Debug(
		"tx gossip received",
		zap.Int("txs", len(txs)),
		zap.Int("previously seen", seen),
		zap.Stringer("nodeID", nodeID),
		zap.Bool("validator", isValidator),
		zap.Duration("t", time.Since(start)),
	)

	// only trace error to prevent VM's being shutdown
	// from "AppGossip" returning an error
	return nil
}

func (g *Proposer) notify() {
	select {
	case g.q <- struct{}{}:
		g.lastQueue = time.Now().UnixMilli()
	default:
	}
}

func (g *Proposer) handleTimerNotify() {
	g.notify()
	g.waiting.Store(false)
}

func (g *Proposer) Queue(context.Context) {
	if !g.waiting.CompareAndSwap(false, true) {
		g.vm.Logger().Debug("unable to start waiting")
		return
	}
	now := time.Now().UnixMilli()
	force := g.lastQueue + g.cfg.GossipMinDelay
	if now >= force {
		g.notify()
		g.waiting.Store(false)
		return
	}
	sleep := force - now
	sleepDur := time.Duration(sleep * int64(time.Millisecond))
	g.timer.SetTimeoutIn(sleepDur)
	g.vm.Logger().Debug("waiting to notify to gossip", zap.Duration("t", sleepDur))
}

// periodically but less aggressively force-regossip the pending
func (g *Proposer) Run(appSender common.AppSender) {
	g.appSender = appSender
	defer close(g.doneGossip)

	// Timer blocks until stopped
	go g.timer.Dispatch()

	for {
		select {
		case <-g.q:
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
					g.Queue(tctx) // requeue later in case peer validator
					g.vm.Logger().Debug("not gossiping because soon to propose")
					continue
				} else if err != nil {
					g.vm.Logger().Warn("unable to determine if will propose soon, gossiping anyways", zap.Error(err))
				}
			}

			// Gossip to proposers who will produce next
			if err := g.Force(tctx); err != nil {
				g.vm.Logger().Warn("gossip txs failed", zap.Error(err))
				continue
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
	g.timer.Stop()
	<-g.doneGossip
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

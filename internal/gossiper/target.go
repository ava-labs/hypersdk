// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossiper

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/timer"
	"github.com/ava-labs/avalanchego/vms/proposervm/proposer"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/internal/cache"
)

var _ Gossiper = (*Target[Tx])(nil)

type Target[T Tx] struct {
	tracer  trace.Tracer
	log     logging.Logger
	metrics *metrics
	mempool Mempool[T]

	serializer           Serializer[T]
	submitter            Submitter[T]
	validatorSet         ValidatorSet
	targetGossipDuration time.Duration

	targetStrategy TargetStrategy[T]
	cfg            *TargetConfig
	client         *p2p.Client
	doneGossip     chan struct{}

	latestVerifiedTimestamp int64

	fl sync.Mutex

	q         chan struct{}
	lastQueue int64
	timer     *timer.Timer
	waiting   atomic.Bool

	// cache is thread-safe
	cache *cache.FIFO[ids.ID, any]

	stop <-chan struct{}
}

type TargetConfig struct {
	GossipMinLife       int64 // ms
	GossipMaxSize       int
	GossipMinDelay      int64 // ms
	NoGossipBuilderDiff int
	VerifyTimeout       int64 // ms
	SeenCacheSize       int
}

type GossipContainer[T any] struct {
	NodeIDs set.Set[ids.NodeID]
	Txs     []T
}

func DefaultTargetConfig() *TargetConfig {
	return &TargetConfig{
		GossipMinLife:       5 * 1000,
		GossipMaxSize:       consts.NetworkSizeLimit,
		GossipMinDelay:      50,
		NoGossipBuilderDiff: 1,
		VerifyTimeout:       proposer.MaxVerifyDelay.Milliseconds(),
		SeenCacheSize:       2_500_000,
	}
}

func NewTarget[T Tx](
	tracer trace.Tracer,
	log logging.Logger,
	registerer prometheus.Registerer,
	mempool Mempool[T],
	serializer Serializer[T],
	submitter Submitter[T],
	validatorSet ValidatorSet,
	targetGossipDuration time.Duration,
	targetStrategy TargetStrategy[T],
	cfg *TargetConfig,
	stop <-chan struct{},
) (*Target[T], error) {
	metrics, err := newMetrics(registerer)
	if err != nil {
		return nil, err
	}
	g := &Target[T]{
		tracer:                  tracer,
		log:                     log,
		metrics:                 metrics,
		mempool:                 mempool,
		serializer:              serializer,
		submitter:               submitter,
		validatorSet:            validatorSet,
		targetStrategy:          targetStrategy,
		targetGossipDuration:    targetGossipDuration,
		cfg:                     cfg,
		stop:                    stop,
		doneGossip:              make(chan struct{}),
		latestVerifiedTimestamp: -1,
		q:                       make(chan struct{}),
		lastQueue:               -1,
	}
	g.timer = timer.NewTimer(g.handleTimerNotify)
	cache, err := cache.NewFIFO[ids.ID, any](cfg.SeenCacheSize)
	if err != nil {
		return nil, err
	}
	g.cache = cache
	return g, nil
}

func (g *Target[T]) Force(ctx context.Context) error {
	ctx, span := g.tracer.Start(ctx, "Gossiper.Force")
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
		txs   = []T{}
		size  = 0
		start = time.Now()
		now   = start.UnixMilli()
	)
	mempoolErr := g.mempool.Top(
		ctx,
		g.targetGossipDuration,
		func(_ context.Context, next T) (cont bool, rest bool, err error) {
			// Remove txs that are expired
			if next.GetExpiry() < now {
				return true, false, nil
			}

			// Don't gossip txs that are about to expire
			life := next.GetExpiry() - now
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
			txID := next.GetID()
			if _, ok := g.cache.Get(txID); ok {
				return true, true, nil
			}
			g.cache.Put(txID, nil)

			txs = append(txs, next)
			size += txSize
			return true, true, nil
		},
	)
	if mempoolErr != nil {
		return mempoolErr
	}
	if len(txs) == 0 {
		g.log.Debug("no transactions to gossip")
		return nil
	}
	g.log.Debug("gossiping transactions", zap.Int("txs", len(txs)), zap.Duration("t", time.Since(start)))
	g.metrics.txsGossiped.Add(float64(len(txs)))
	return g.sendTxs(ctx, txs)
}

func (g *Target[T]) HandleAppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	txs, err := g.serializer.Unmarshal(msg)
	if err != nil {
		g.log.Warn(
			"received invalid txs",
			zap.Stringer("peerID", nodeID),
			zap.Error(err),
		)
		return nil
	}
	g.metrics.txsReceived.Add(float64(len(txs)))
	var seen int
	for _, tx := range txs {
		// Add incoming txs to the cache to make
		// sure we never gossip anything we receive (someone
		// else will)
		if g.cache.Put(tx.GetID(), nil) {
			seen++
		}
	}
	g.metrics.seenTxsReceived.Add(float64(seen))

	// Mark incoming gossip as held by [nodeID], if it is a validator
	isValidator, err := g.validatorSet.IsValidator(ctx, nodeID)
	if err != nil {
		g.log.Warn(
			"unable to determine if nodeID is validator",
			zap.Stringer("peerID", nodeID),
			zap.Error(err),
		)
	}

	// Submit incoming gossip to mempool
	start := time.Now()
	numErrs := 0
	for _, err := range g.submitter.Submit(ctx, txs) {
		if err != nil {
			numErrs++
		}
	}
	g.log.Debug(
		"tx gossip received",
		zap.Int("txs", len(txs)),
		zap.Int("numFailedSubmit", numErrs),
		zap.Int("previously seen", seen),
		zap.Stringer("nodeID", nodeID),
		zap.Bool("validator", isValidator),
		zap.Duration("t", time.Since(start)),
	)

	// only trace error to prevent VM's being shutdown
	// from "AppGossip" returning an error
	return nil
}

func (g *Target[T]) notify() {
	select {
	case g.q <- struct{}{}:
		g.lastQueue = time.Now().UnixMilli()
	default:
	}
}

func (g *Target[T]) handleTimerNotify() {
	g.notify()
	g.waiting.Store(false)
}

func (g *Target[T]) Queue(context.Context) {
	if !g.waiting.CompareAndSwap(false, true) {
		g.log.Debug("unable to start waiting")
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
	g.log.Debug("waiting to notify to gossip", zap.Duration("t", sleepDur))
}

// periodically but less aggressively force-regossip the pending
func (g *Target[T]) Start(client *p2p.Client) {
	g.client = client

	// Timer blocks until stopped
	go g.timer.Dispatch()
	go func() {
		defer close(g.doneGossip)

		for {
			select {
			case <-g.q:
				tctx := context.Background()

				// Check if we are going to propose if it has been less than
				// [VerifyTimeout] since the last time we verified a block.
				if time.Now().UnixMilli()-g.latestVerifiedTimestamp < g.cfg.VerifyTimeout {
					proposers, err := g.validatorSet.Proposers(
						tctx,
						g.cfg.NoGossipBuilderDiff,
						1,
					)
					if err == nil && proposers.Contains(g.validatorSet.NodeID()) {
						g.Queue(tctx) // requeue later in case peer validator
						g.log.Debug("not gossiping because soon to propose")
						continue
					} else if err != nil {
						g.log.Warn("unable to determine if will propose soon, gossiping anyways", zap.Error(err))
					}
				}

				// Gossip to targeted nodes
				if err := g.Force(tctx); err != nil {
					g.log.Warn("gossip txs failed", zap.Error(err))
					continue
				}
			case <-g.stop:
				g.log.Info("stopping gossip loop")
				return
			}
		}
	}()
}

func (g *Target[T]) BlockVerified(t int64) {
	if t < g.latestVerifiedTimestamp {
		return
	}
	g.latestVerifiedTimestamp = t
}

func (g *Target[T]) Done() {
	g.timer.Stop()
	<-g.doneGossip
}

func (g *Target[T]) sendTxs(ctx context.Context, txs []T) error {
	ctx, span := g.tracer.Start(ctx, "Gossiper.sendTxs")
	defer span.End()

	gossipContainers, err := g.targetStrategy.Target(ctx, txs)
	if err != nil {
		return err
	}

	for _, gossipContainer := range gossipContainers {
		// Marshal gossip
		txBatchBytes := g.serializer.Marshal(gossipContainer.Txs)

		// Send gossip to specified peers
		if err := g.client.AppGossip(ctx, common.SendConfig{NodeIDs: gossipContainer.NodeIDs}, txBatchBytes); err != nil {
			return err
		}
	}
	return nil
}

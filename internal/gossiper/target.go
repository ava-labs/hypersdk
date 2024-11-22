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
	t := &Target[T]{
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
	t.timer = timer.NewTimer(t.handleTimerNotify)
	cache, err := cache.NewFIFO[ids.ID, any](cfg.SeenCacheSize)
	if err != nil {
		return nil, err
	}
	t.cache = cache
	return t, nil
}

func (t *Target[T]) Force(ctx context.Context) error {
	ctx, span := t.tracer.Start(ctx, "Gossiper.Force")
	defer span.End()

	t.fl.Lock()
	defer t.fl.Unlock()

	// Select txs eligible to gossip from the mempool and which
	// should be removed from the local mempool.
	var (
		txs   = []T{}
		size  = 0
		start = time.Now()
		now   = start.UnixMilli()
	)
	mempoolErr := t.mempool.Top(
		ctx,
		t.targetGossipDuration,
		func(_ context.Context, next T) (cont bool, rest bool, err error) {
			// Remove txs that are expired
			if next.Expiry() < now {
				return true, false, nil
			}

			// Don't gossip txs that are about to expire
			life := next.Expiry() - now
			if life < t.cfg.GossipMinLife {
				return true, true, nil
			}

			// Gossip up to [GossipMaxSize]
			txSize := next.Size()
			if txSize+size > t.cfg.GossipMaxSize {
				return false, true, nil
			}

			// Don't remove anything from mempool
			// that will be dropped (this seems
			// like we sent it then got sent it back?)
			txID := next.ID()
			if _, ok := t.cache.Get(txID); ok {
				return true, true, nil
			}
			t.cache.Put(txID, nil)

			txs = append(txs, next)
			size += txSize
			return true, false, nil
		},
	)
	t.metrics.selectTxsToGossipCount.Inc()
	t.metrics.selectTxsToGossipSum.Add(float64(time.Since(start)))
	t.metrics.selectedTxsToGossip.Add(float64(len(txs)))
	if mempoolErr != nil {
		return mempoolErr
	}
	if len(txs) == 0 {
		t.log.Debug("no transactions to gossip")
		return nil
	}
	return t.sendTxs(ctx, txs)
}

func (t *Target[T]) HandleAppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	txs, err := t.serializer.Unmarshal(msg)
	if err != nil {
		t.log.Warn(
			"received invalid txs",
			zap.Stringer("peerID", nodeID),
			zap.Error(err),
		)
		return nil
	}
	t.metrics.txsReceived.Add(float64(len(txs)))
	var seen int
	for _, tx := range txs {
		// Add incoming txs to the cache to make
		// sure we never gossip anything we receive (someone
		// else will)
		if t.cache.Put(tx.ID(), nil) {
			seen++
		}
	}
	t.metrics.seenTxsReceived.Add(float64(seen))

	// Mark incoming gossip as held by [nodeID], if it is a validator
	isValidator, err := t.validatorSet.IsValidator(ctx, nodeID)
	if err != nil {
		t.log.Warn(
			"unable to determine if nodeID is validator",
			zap.Stringer("peerID", nodeID),
			zap.Error(err),
		)
	}

	// Submit incoming gossip to mempool
	start := time.Now()
	numErrs := 0
	for _, err := range t.submitter.Submit(ctx, txs) {
		if err != nil {
			numErrs++
		}
	}
	t.log.Debug(
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

func (t *Target[T]) notify() {
	select {
	case t.q <- struct{}{}:
		t.lastQueue = time.Now().UnixMilli()
	default:
	}
}

func (t *Target[T]) handleTimerNotify() {
	t.notify()
	t.waiting.Store(false)
}

func (t *Target[T]) Queue(context.Context) {
	if !t.waiting.CompareAndSwap(false, true) {
		t.log.Debug("unable to start waiting")
		return
	}
	now := time.Now().UnixMilli()
	force := t.lastQueue + t.cfg.GossipMinDelay
	if now >= force {
		t.notify()
		t.waiting.Store(false)
		return
	}
	sleep := force - now
	sleepDur := time.Duration(sleep * int64(time.Millisecond))
	t.timer.SetTimeoutIn(sleepDur)
	t.log.Debug("waiting to notify to gossip", zap.Duration("t", sleepDur))
}

// periodically but less aggressively force-regossip the pending
func (t *Target[T]) Run(client *p2p.Client) {
	t.client = client
	defer close(t.doneGossip)

	// Timer blocks until stopped
	go t.timer.Dispatch()

	for {
		select {
		case <-t.q:
			tctx := context.Background()

			// Check if we are going to propose if it has been less than
			// [VerifyTimeout] since the last time we verified a block.
			if time.Now().UnixMilli()-t.latestVerifiedTimestamp < t.cfg.VerifyTimeout {
				proposers, err := t.validatorSet.Proposers(
					tctx,
					t.cfg.NoGossipBuilderDiff,
					1,
				)
				if err == nil && proposers.Contains(t.validatorSet.NodeID()) {
					t.Queue(tctx) // requeue later in case peer validator
					t.log.Debug("not gossiping because soon to propose")
					continue
				} else if err != nil {
					t.log.Warn("unable to determine if will propose soon, gossiping anyways", zap.Error(err))
				}
			}

			// Gossip to targeted nodes
			if err := t.Force(tctx); err != nil {
				t.log.Warn("gossip txs failed", zap.Error(err))
				continue
			}
		case <-t.stop:
			t.log.Info("stopping gossip loop")
			return
		}
	}
}

func (t *Target[T]) BlockVerified(verifiedTimestamp int64) {
	if verifiedTimestamp < t.latestVerifiedTimestamp {
		return
	}
	t.latestVerifiedTimestamp = verifiedTimestamp
}

func (t *Target[T]) Done() {
	t.timer.Stop()
	<-t.doneGossip
}

func (t *Target[T]) sendTxs(ctx context.Context, txs []T) error {
	ctx, span := t.tracer.Start(ctx, "Gossiper.sendTxs")
	defer span.End()

	start := time.Now()
	gossipContainers, err := t.targetStrategy.Target(ctx, txs)
	if err != nil {
		return err
	}

	numTxsGossipped := 0
	for _, gossipContainer := range gossipContainers {
		numTxsGossipped += len(gossipContainer.Txs)
		b, err := t.serializer.Marshal(gossipContainer.Txs)
		if err != nil {
			return err
		}

		// Send gossip to specified peers
		if err := t.client.AppGossip(ctx, common.SendConfig{NodeIDs: gossipContainer.NodeIDs}, b); err != nil {
			return err
		}
	}
	t.metrics.txsGossiped.Add(float64(numTxsGossipped))
	t.metrics.targetTxsSum.Add(float64(time.Since(start)))
	return nil
}

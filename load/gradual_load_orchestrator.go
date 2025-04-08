// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var (
	_ Orchestrator = (*GradualLoadOrchestrator[any, any])(nil)

	ErrFailedToReachTargetTPS = errors.New("failed to reach target TPS")
)

type GradualLoadOrchestratorConfig struct {
	// the maximum TPS the orchestrator should aim for.
	MaxTPS uint64
	// the minimum TPS the orchestrator should start with.
	MinTPS uint64
	// the step size to increase the TPS by.
	Step uint64

	// the factor by which to pad the number of txs an issuer sends per second
	// for example, if targetTPS = 1000 and numIssuers = 10, then each issuer
	// will send (1000/10)*TxRateMultiplier transactions per second.
	//
	// this is useful when the network can handle the current target TPS but it
	// hasn't received enough transactions to achieve that TPS (as a result of
	// latency between individual issuers and the API nodes).
	TxRateMultiplier float64

	// the time period which TPS is averaged over
	// similarly, the time period which the orchestrator will wait before
	// computing the average TPS.
	SustainedTime time.Duration
	// the number of attempts to try achieving a given target TPS before giving up.
	MaxAttempts uint64

	// whether the orchestrator should return if the maxTPS has been reached
	Terminate bool
}

func DefaultGradualLoadOrchestratorConfig() GradualLoadOrchestratorConfig {
	return GradualLoadOrchestratorConfig{
		MaxTPS:           5_000,
		MinTPS:           1_000,
		Step:             1_000,
		TxRateMultiplier: 1.3,
		SustainedTime:    20 * time.Second,
		MaxAttempts:      3,
		Terminate:        true,
	}
}

// GradualLoadOrchestrator tests the network by continuously sending
// transactions at a given rate (currTargetTPS) and increasing that rate until it detects that
// the network can no longer make progress (i.e. the rate at the network accepts
// transactions is less than currTargetTPS).
type GradualLoadOrchestrator[T, U comparable] struct {
	generators []TxGenerator[T]
	issuers    []Issuer[T]
	tracker    Tracker[U]

	log logging.Logger

	maxObservedTPS atomic.Uint64

	observerGroup errgroup.Group
	issuerGroup   *errgroup.Group

	config GradualLoadOrchestratorConfig
}

func NewGradualLoadOrchestrator[T, U comparable](
	generators []TxGenerator[T],
	issuers []Issuer[T],
	tracker Tracker[U],
	log logging.Logger,
	config GradualLoadOrchestratorConfig,
) (*GradualLoadOrchestrator[T, U], error) {
	if len(generators) != len(issuers) {
		return nil, ErrMismatchedGeneratorsAndIssuers
	}
	return &GradualLoadOrchestrator[T, U]{
		generators: generators,
		issuers:    issuers,
		tracker:    tracker,
		log:        log,
		config:     config,
	}, nil
}

func (o *GradualLoadOrchestrator[T, U]) Execute(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)

	// start a goroutine to confirm each issuer's transactions
	for _, issuer := range o.issuers {
		o.observerGroup.Go(func() error { return issuer.Listen(ctx) })
	}

	// start the test and block until it's done
	success := o.run(ctx)

	var err error
	if !success {
		err = ErrFailedToReachTargetTPS
	}

	// stop the observers and issuers
	cancel()

	// block until both the observers and issuers have stopped
	return errors.Join(o.issuerGroup.Wait(), o.observerGroup.Wait(), err)
}

// run the gradual load test by continuously increasing the rate at which
// transactions are sent
//
// run blocks until one of the following conditions is met:
//
// 1. an issuer has errored
// 2. the max TPS target has been reached and we can terminate
// 3. the maximum number of attempts to reach a target TPS has been reached
func (o *GradualLoadOrchestrator[T, U]) run(ctx context.Context) bool {
	var (
		prevConfirmed = o.tracker.GetObservedConfirmed()
		prevTime      = time.Now()
		currTargetTPS = new(atomic.Uint64)
		// true if the orchestrator has reached the max TPS target
		achievedTargetTPS bool
		attempts          uint64
	)

	currTargetTPS.Store(o.config.MinTPS)

	issuerGroup, issuerCtx := errgroup.WithContext(ctx)
	o.issuerGroup = issuerGroup

	o.issueTxs(issuerCtx, currTargetTPS)

	for {
		// wait for the sustained time to pass or for the context to be cancelled
		select {
		case <-time.After(o.config.SustainedTime):
		case <-issuerCtx.Done(): // the parent context was cancelled or an issuer errored
		}

		if issuerCtx.Err() != nil {
			break // Case 1
		}

		currConfirmed := o.tracker.GetObservedConfirmed()
		currTime := time.Now()

		tps := computeTPS(prevConfirmed, currConfirmed, currTime.Sub(prevTime))
		o.setMaxObservedTPS(tps)

		// if max TPS target has been reached and we don't terminate, then continue so
		// that we do not manipulate TPS target
		if achievedTargetTPS && !o.config.Terminate {
			o.log.Info(
				"current network state",
				zap.Uint64("current TPS", tps),
				zap.Uint64("max observed TPS", o.maxObservedTPS.Load()),
			)
			continue
		}

		if tps >= currTargetTPS.Load() {
			if currTargetTPS.Load() >= o.config.MaxTPS {
				achievedTargetTPS = true
				o.log.Info(
					"max TPS target reached",
					zap.Uint64("max TPS target", currTargetTPS.Load()),
					zap.Uint64("average TPS", tps),
				)
				if o.config.Terminate {
					o.log.Info("terminating orchestrator")
					break // Case 2
				} else {
					o.log.Info("orchestrator will now continue running at max TPS")
					continue
				}
			}
			o.log.Info(
				"increasing TPS",
				zap.Uint64("previous target TPS", currTargetTPS.Load()),
				zap.Uint64("average TPS", tps),
				zap.Uint64("new target TPS", currTargetTPS.Load()+o.config.Step),
			)
			currTargetTPS.Add(o.config.Step)
			attempts = 0
		} else {
			if attempts >= o.config.MaxAttempts {
				o.log.Info(
					"max attempts reached",
					zap.Uint64("attempted target TPS", currTargetTPS.Load()),
					zap.Uint64("number of attempts", attempts),
				)
				break // Case 3
			}
			o.log.Info(
				"failed to reach target TPS, retrying",
				zap.Uint64("current target TPS", currTargetTPS.Load()),
				zap.Uint64("average TPS", tps),
				zap.Uint64("attempt number", attempts),
			)
			attempts++
		}

		prevConfirmed = currConfirmed
		prevTime = currTime
	}

	return achievedTargetTPS
}

// GetObservedIssued returns the max TPS the orchestrator observed
func (o *GradualLoadOrchestrator[T, U]) GetMaxObservedTPS() uint64 {
	return o.maxObservedTPS.Load()
}

// start a goroutine to each issuer to continuously send transactions
// if an issuer errors, all other issuers will stop as well.
func (o *GradualLoadOrchestrator[T, U]) issueTxs(ctx context.Context, currTargetTPS *atomic.Uint64) {
	for i, issuer := range o.issuers {
		o.issuerGroup.Go(func() error {
			for {
				select {
				case <-ctx.Done():
					return nil
				default:
				}
				currTime := time.Now()
				txsPerIssuer := uint64(math.Ceil(float64(currTargetTPS.Load())/float64(len(o.issuers))) * o.config.TxRateMultiplier)
				for range txsPerIssuer {
					tx, err := o.generators[i].GenerateTx(ctx)
					if err != nil {
						return err
					}
					if err := issuer.IssueTx(ctx, tx); err != nil {
						return err
					}
				}
				diff := time.Second - time.Since(currTime)
				if diff > 0 {
					time.Sleep(diff)
				}
			}
		})
	}
}

// setMaxObservedTPS only if tps > the current max observed TPS.
func (o *GradualLoadOrchestrator[T, U]) setMaxObservedTPS(tps uint64) {
	if tps > o.maxObservedTPS.Load() {
		o.maxObservedTPS.Store(tps)
	}
}

func computeTPS(initial uint64, final uint64, duration time.Duration) uint64 {
	seconds := uint64(duration.Seconds())
	if seconds == 0 {
		return 0
	} else {
		return (final - initial) / seconds
	}
}

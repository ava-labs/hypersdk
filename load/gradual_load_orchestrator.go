// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"errors"
	"math"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

var _ Orchestrator = (*GradualLoadOrchestrator[any, any])(nil)

type GradualLoadOrchestratorConfig struct {
	// the maximum TPS the orchestrator should aim for.
	MaxTPS uint64
	// the minimum TPS the orchestrator should start with.
	MinTPS uint64
	// the step size to increase the TPS by.
	Step uint64

	// the factor by which to pad the number of TXs an issuer sends per second
	// for example, if targetTPS = 1000 and numOfIssuers = 10, then each issuer
	// will send (1000/10)*TxMultiplier transactions per second.
	//
	// this is useful when the network can handle the current target TPS but it
	// hasn't received enough transactions to achieve that TPS (as a result of
	// latency + other factors).
	TxMultiplier float64

	// the time period which TPS is averaged over
	// similarly, the time period which the orchestrator will wait before
	// computing the average TPS.
	SustainedTime time.Duration
	// the number of attempts to try achieving a given target TPS before giving up.
	MaxAttempts uint64
}

func DefaultGradualLoadOrchestratorConfig() GradualLoadOrchestratorConfig {
	return GradualLoadOrchestratorConfig{
		MaxTPS:        80_000,
		MinTPS:        1_000,
		Step:          1_000,
		TxMultiplier:  1.3,
		SustainedTime: 20 * time.Second,
		MaxAttempts:   3,
	}
}

// GradualLoadOrchestrator tests the network by continuously sending
// transactions at a given rate (targetTPS) and increasing that rate until it detects that
// the network can no longer make progress (i.e. the rate at the network accepts
// transactions is less than targetTPS).
type GradualLoadOrchestrator[T, U comparable] struct {
	generators []TxGenerator[T]
	issuers    []Issuer[T]
	tracker    Tracker[U]

	maxObservedTPS uint64

	observerGroup errgroup.Group
	issuerGroup   *errgroup.Group

	done chan struct{}

	cancel context.CancelFunc

	config GradualLoadOrchestratorConfig
}

func NewGradualLoadOrchestrator[T, U comparable](
	generators []TxGenerator[T],
	issuers []Issuer[T],
	tracker Tracker[U],
	config GradualLoadOrchestratorConfig,
) (*GradualLoadOrchestrator[T, U], error) {
	if len(generators) != len(issuers) {
		return nil, ErrMismatchedGeneratorsAndIssuers
	}
	return &GradualLoadOrchestrator[T, U]{
		generators: generators,
		issuers:    issuers,
		tracker:    tracker,
		config:     config,
		done:       make(chan struct{}),
	}, nil
}

func (o *GradualLoadOrchestrator[T, U]) Execute(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	o.cancel = cancel

	// start a goroutine to confirm each issuer's transactions
	for _, issuer := range o.issuers {
		o.observerGroup.Go(func() error { return issuer.Listen(ctx) })
	}

	go o.run(ctx)

	// wait for the test to finish
	<-o.done

	// stop the observers and issuers
	o.cancel()

	// block until both the observers and issuers have stopped
	return errors.Join(o.issuerGroup.Wait(), o.observerGroup.Wait())
}

// run the gradual load test by continuously increasing the rate at which
// transactions are sent
//
// run stops when the network can no longer make progress or if an issuer errors.
func (o *GradualLoadOrchestrator[T, U]) run(ctx context.Context) {
	defer close(o.done)

	var (
		prevConfirmed = o.tracker.GetObservedConfirmed()
		prevTime      = time.Now()
		attempts      uint64
	)

	currTargetTPS := atomic.Uint64{}
	currTargetTPS.Store(o.config.MinTPS)

	issuerGroup, issuerCtx := errgroup.WithContext(ctx)
	o.issuerGroup = issuerGroup

	// start a goroutine to each issuer to continuously send transactions
	// if an issuer errors, all other issuers will stop as well.
	for i, issuer := range o.issuers {
		o.issuerGroup.Go(func() error {
			for {
				select {
				case <-issuerCtx.Done():
					return issuerCtx.Err()
				default:
				}
				currTime := time.Now()
				txsPerIssuer := uint64(math.Ceil(float64(currTargetTPS.Load())/float64(len(o.issuers))) * o.config.TxMultiplier)
				for range txsPerIssuer {
					tx, err := o.generators[i].GenerateTx(issuerCtx)
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

	// blocks until either 1) the max TPS target has been reached, 2) we've
	// maxed out the number of attempts, or 3) an issuer has errored.
	for {
		time.Sleep(o.config.SustainedTime)

		currConfirmed := o.tracker.GetObservedConfirmed()
		currTime := time.Now()

		if issuerCtx.Err() != nil {
			break
		}

		tps := computeTPS(prevConfirmed, currConfirmed, currTime.Sub(prevTime))
		if tps >= currTargetTPS.Load() {
			if currTargetTPS.Load() >= o.config.MaxTPS {
				break
			}
			currTargetTPS.Add(o.config.Step)
			attempts = 0
		} else {
			if attempts >= o.config.MaxAttempts {
				break
			}
			attempts++
		}

		prevConfirmed = currConfirmed
		prevTime = currTime
	}

	o.maxObservedTPS = currTargetTPS.Load() - o.config.Step
}

func (o *GradualLoadOrchestrator[T, U]) GetMaxObservedTPS() uint64 {
	return o.maxObservedTPS
}

func computeTPS(initial uint64, final uint64, duration time.Duration) uint64 {
	seconds := uint64(duration.Seconds())
	if seconds == 0 {
		return 0
	} else {
		return (final - initial) / seconds
	}
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"
)

var _ orchestrator = (*ShortBurstOrchestrator[any, any])(nil)

type ShortBurstOrchestratorConfig struct {
	TxsPerIssuer uint64
	Timeout      time.Duration
}

// ShortBurstOrchestrator tests the network by sending a fixed number of
// transactions en masse in a short timeframe.
type ShortBurstOrchestrator[T, U comparable] struct {
	agents []Agent[T, U]
	log    Logger

	config ShortBurstOrchestratorConfig
}

func NewShortBurstOrchestrator[T, U comparable](
	agents []Agent[T, U],
	log Logger,
	config ShortBurstOrchestratorConfig,
) (*ShortBurstOrchestrator[T, U], error) {
	return &ShortBurstOrchestrator[T, U]{
		agents: agents,
		log:    log,
		config: config,
	}, nil
}

// Execute orders issuers to send a fixed number of transactions and then waits
// for all of their statuses to be confirmed or for a timeout to occur.
func (o *ShortBurstOrchestrator[T, U]) Execute(ctx context.Context) error {
	observerCtx, observerCancel := context.WithCancel(ctx)
	defer observerCancel()

	// start a goroutine to confirm each issuer's transactions
	observerGroup := errgroup.Group{}
	for _, agent := range o.agents {
		observerGroup.Go(func() error { return agent.Listener.Listen(observerCtx) })
	}

	const logInterval = 10 * time.Second
	logTicker := time.NewTicker(logInterval)

	// start issuing transactions sequentially from each issuer
	issuerGroup := errgroup.Group{}
	for _, agent := range o.agents {
		issuerGroup.Go(func() error {
			for i := range o.config.TxsPerIssuer {
				tx, err := agent.Issuer.GenerateAndIssueTx(ctx)
				if err != nil {
					return err
				}
				lastIssued := i == o.config.TxsPerIssuer-1
				agent.Listener.RegisterIssued(tx, lastIssued)

				select {
				case <-logTicker.C:
					o.log.Info(agent.Tracker.String())
				default:
				}
			}
			return nil
		})
	}

	// wait for all issuers to finish sending their transactions
	if err := issuerGroup.Wait(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, o.config.Timeout)
	defer cancel()

	// start a goroutine that will cancel the observer group's context
	// if either the parent context is cancelled or our timeout elapses
	go func() {
		<-ctx.Done()
		observerCancel()
	}()

	// blocks until either all of the issuers have finished or our context
	// is cancelled signalling for early termination (with an error)
	return observerGroup.Wait()
}

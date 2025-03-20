// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"time"

	"golang.org/x/sync/errgroup"
)

var _ Orchestrator[any] = (*ShortBurstOrchestrator[any])(nil)

type ShortBurstOrchestratorConfig struct {
	N       uint64
	Timeout time.Duration
}

// ShortBurstOrchestrator is an orchestrator that orders issuers to continue
// sending TXs until the tx generators have been exhausted or if an error occurs
type ShortBurstOrchestrator[T comparable] struct {
	generators []TxGenerator[T]
	issuers    []Issuer[T]
	tracker    Tracker

	issuerGroup   errgroup.Group
	observerGroup errgroup.Group

	cancel context.CancelFunc

	config ShortBurstOrchestratorConfig
}

func NewShortBurstOrchestrator[T comparable](
	txGenerators []TxGenerator[T],
	issuers []Issuer[T],
	tracker Tracker,
	config ShortBurstOrchestratorConfig,
) *ShortBurstOrchestrator[T] {
	return &ShortBurstOrchestrator[T]{
		generators: txGenerators,
		issuers:    issuers,
		tracker:    tracker,
		config:     config,
	}
}

// The orchestrator is responsible for determining the number of TXs to send
func (o *ShortBurstOrchestrator[T]) Execute(ctx context.Context) error {
	observerCtx, cancel := context.WithCancel(ctx)
	o.cancel = cancel

	// command issuers to start listening to their transactions
	for _, issuer := range o.issuers {
		o.observerGroup.Go(func() error { return issuer.Listen(observerCtx) })
	}

	// command issuers to start sending transactions
	for i, issuer := range o.issuers {
		generator := o.generators[i]
		o.issuerGroup.Go(func() error {
			defer issuer.Stop()

			for range o.config.N {
				tx, err := generator.GenerateTx(ctx)
				if err != nil {
					return err
				}
				if err := issuer.IssueTx(ctx, tx); err != nil {
					return err
				}
			}
			return nil
		})
	}

	// wait for all issuers to finish sending their transactions
	if err := o.issuerGroup.Wait(); err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(ctx, o.config.Timeout)
	defer cancel()

	// start a goroutine that will cancel the observer group's context
	// if either the parent context is cancelled or our timeout elapses
	go func() {
		<-ctx.Done()
		o.cancel()
	}()

	// blocks until either all of the issuers have finished or our context
	// is cancelled signalling for early termination (with an error)
	return o.observerGroup.Wait()
}

// TODOs
// create README that clearly describes breakdown of components
// finish implementing the burst orchestrator - issue N txs from each issuer/generator and wait for all to complete
// ensure high quality of logs and metrics
// logs for issuance + confirmation + failure, startup/termination of each goroutine, and startup/termination of overall process
// metrics as we defined:
// txsIssued
// txsConfirmed
// txsFailed
// this can be used to track issued outstanding as well
// latency for confirmed/failed
// Grafana dashboard for client side load generation metrics
// Grafana dashboard for network side txs_accepted (TPS)

// replace Tx type constraint with T = comparable (GetID should not be needed?)
// ✅

// Update documentation / comments on dependencies ✅

// Update TxGenerator to remove exhausted (if the tx generator does not signal
// when it's done, then this should not exist) ✅

// move issuer implementation into load/hyper/ package ie. hypersdk specific ✅

// combine Start/Wait into a single Execute function for the orchestrator ✅

// complete all TODOs throughout the change (ie. complete all TODO configurable
// ie. number of txs, duration of issuance, timeout duration, etc.)

// create README + mermaid diagram explaining the architecture

// update CI throughput test to use load package instead of throughput

// create draft PR and re-sync

// implement gradual load orchestrator based on agreed on interfaces and re-sync

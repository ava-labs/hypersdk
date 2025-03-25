// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"errors"
	"time"

	"golang.org/x/sync/errgroup"
)

var (
	ErrMismatchedGeneratorsAndIssuers = errors.New("number of generators and issuers must match")

	_ Orchestrator = (*ShortBurstOrchestrator[any, any])(nil)
)

type ShortBurstOrchestratorConfig struct {
	TxsPerIssuer uint64
	Timeout      time.Duration
}

// ShortBurstOrchestrator tests the network by sending a fixed number of
// transactions en masse in a short timeframe.
type ShortBurstOrchestrator[T, U comparable] struct {
	generators []TxGenerator[T]
	issuers    []Issuer[T]
	tracker    Tracker[U]

	issuerGroup   errgroup.Group
	observerGroup errgroup.Group

	config ShortBurstOrchestratorConfig
}

func NewShortBurstOrchestrator[T, U comparable](
	txGenerators []TxGenerator[T],
	issuers []Issuer[T],
	tracker Tracker[U],
	config ShortBurstOrchestratorConfig,
) (*ShortBurstOrchestrator[T, U], error) {
	if len(txGenerators) != len(issuers) {
		return nil, ErrMismatchedGeneratorsAndIssuers
	}
	return &ShortBurstOrchestrator[T, U]{
		generators: txGenerators,
		issuers:    issuers,
		tracker:    tracker,
		config:     config,
	}, nil
}

// Execute orders issuers to send a fixed number of transactions and then waits
// for all of their statuses to be confirmed or for a timeout to occur.
func (o *ShortBurstOrchestrator[T, U]) Execute(ctx context.Context) error {
	observerCtx, observerCancel := context.WithCancel(ctx)
	defer observerCancel()

	// start a goroutine to confirm each issuer's transactions
	for _, issuer := range o.issuers {
		o.observerGroup.Go(func() error { return issuer.Listen(observerCtx) })
	}

	// start issuing transactions sequentially from each issuer
	for i, issuer := range o.issuers {
		generator := o.generators[i]
		o.issuerGroup.Go(func() error {
			defer issuer.Stop()

			for range o.config.TxsPerIssuer {
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
	return o.observerGroup.Wait()
}

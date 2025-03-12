// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
)

type TxGenerator[T fmt.Stringer] interface {
	// GenerateTx generates a new transactin that is expected to be
	// valid at the time of generation.
	// Returns whether the generator is exhausted.
	GenerateTx(ctx context.Context) (T, bool)
}

type TxReceipt[T fmt.Stringer] struct {
	Tx      T
	Success bool
}

type Issuer[T fmt.Stringer] interface {
	IssueTx(ctx context.Context, tx T) error
}

type Orchestrator interface {
	Start(ctx context.Context) error
	Wait(ctx context.Context) error
}

// Tracker provides all client side load generation metrics.
// This must be thread-safe, so it can be called in parallel by the issuer or orchestrator.
type Tracker[T fmt.Stringer] interface {
	IncrementTxIssued()
	ObserveConfirmed(latency time.Duration)
	ObserveFailed(latency time.Duration)
}

// how do we get the source of truth from the nodes themselves?
// make sure to avoid a bug like last time

type shortBurstOrchestrator[T fmt.Stringer] struct {
	generators []TxGenerator[T]
	issuers    []Issuer[T]

	eg errgroup.Group
}

func NewShortBurstOrchestrator[T fmt.Stringer](
	txGenerators []TxGenerator[T],
	issuers []Issuer[T],
) *shortBurstOrchestrator[T] {
	return &shortBurstOrchestrator[T]{
		generators: txGenerators,
		issuers:    issuers,
	}
}

func (o *shortBurstOrchestrator[T]) Start(ctx context.Context) error {
	for i, issuer := range o.issuers {
		generator := o.generators[i]
		o.eg.Go(func() error {
			for {
				tx, more := generator.GenerateTx(ctx)
				if !more {
					return nil
				}
				if err := issuer.IssueTx(ctx, tx); err != nil {
					return err
				}
			}
		})
	}
	return nil
}

func (o *shortBurstOrchestrator[T]) Wait(_ context.Context) error {
	return o.eg.Wait()
}

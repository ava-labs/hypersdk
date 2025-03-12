// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"fmt"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"golang.org/x/sync/errgroup"
)

type Tx interface {
	fmt.Stringer
	GetID() ids.ID
}

type TxGenerator[T Tx] interface {
	// GenerateTx generates a new transactin that is expected to be
	// valid at the time of generation.
	// Returns whether the generator is exhausted.
	GenerateTx(ctx context.Context) (T, bool)
}

type TxReceipt[T Tx] struct {
	Tx      T
	Success bool
}

type Issuer[T Tx] interface {
	IssueTx(ctx context.Context, tx T) error
}

type Orchestrator interface {
	Start(ctx context.Context) error
	Wait(ctx context.Context) error
}

// Tracker provides all client side load generation metrics.
// This must be thread-safe, so it can be called in parallel by the issuer or orchestrator.
type Tracker[T Tx] interface {
	Issue(tx T)
	ObserveConfirmed(tx T)
	ObserveFailed(tx T)

	// What are the latest high level statistics for the orchestrator?
	// Getters for Txs Issued/Confirmed/Failed + TPS over last 10s
}

type tracker[T Tx] struct {
	// TODO: make thread safe
	outstandingTxs map[ids.ID]time.Time
}

func (t *tracker[T]) Issue(tx T) {
	t.outstandingTxs[tx.GetID()] = time.Now()
}

func (t *tracker[T]) ObserveConfirmed(tx T) {
	issued := t.outstandingTxs[tx.GetID()]
	delete(t.outstandingTxs, tx.GetID())
	elapsed := time.Since(issued)

	// log + emit metrics
	_ = elapsed
}

func (t *tracker[T]) ObserveFailed(tx T) {
	issued := t.outstandingTxs[tx.GetID()]
	delete(t.outstandingTxs, tx.GetID())
	elapsed := time.Since(issued)

	// log + emit metrics
	_ = elapsed
}

// how do we get the source of truth from the nodes themselves?
// make sure to avoid a bug like last time

type shortBurstOrchestrator[T Tx] struct {
	generators []TxGenerator[T]
	issuers    []Issuer[T]

	// issue N txs from each generator in parallel
	// and wait for all to complete (N is a parameter)

	eg errgroup.Group
}

func NewShortBurstOrchestrator[T Tx](
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

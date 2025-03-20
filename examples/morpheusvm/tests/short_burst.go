// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tests

import (
	"context"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/load"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/vm"
	"github.com/ava-labs/hypersdk/tests/registry"
	"github.com/ava-labs/hypersdk/tests/workload"

	hload "github.com/ava-labs/hypersdk/load"
)

func ShortBurstTest(t require.TestingT, tn workload.TestNetwork) {
	r := require.New(t)
	ctx := context.Background()

	lcli := vm.NewJSONRPCClient(tn.URIs()[0])
	ruleFactory, err := lcli.GetRuleFactory(ctx)
	r.NoError(err)

	numOfFactories := len(tn.Configuration().AuthFactories())
	balances := make([]uint64, numOfFactories)
	// Get balances
	for i, factory := range tn.Configuration().AuthFactories() {
		balance, err := lcli.Balance(ctx, factory.Address())
		r.NoError(err)
		balances[i] = balance
	}

	cli := jsonrpc.NewJSONRPCClient(tn.URIs()[0])
	unitPrices, err := cli.UnitPrices(ctx, false)
	r.NoError(err)

	// Create tx generator
	txGenerators := make([]hload.TxGenerator[*chain.Transaction], numOfFactories)
	for i := 0; i < numOfFactories; i++ {
		txGenerators[i] = load.NewTxGenerator(tn.Configuration().AuthFactories()[i], ruleFactory, balances[i], unitPrices)
	}

	// Create tracker
	registry := prometheus.NewRegistry()
	tracker := hload.NewPrometheusTracker(registry)

	// Create issuers
	issuers := make([]hload.Issuer[*chain.Transaction], numOfFactories)
	for i := 0; i < numOfFactories; i++ {
		issuer, err := hload.NewDefaultIssuer(tn.URIs()[0], tracker)
		r.NoError(err)
		issuers[i] = issuer
	}

	numOfTxsPerIssuer := uint64(1_000)
	numOfTxs := numOfTxsPerIssuer * uint64(numOfFactories)

	// Create orchestrator
	orchestrator := hload.NewShortBurstOrchestrator(
		txGenerators,
		issuers,
		tracker,
		hload.ShortBurstOrchestratorConfig{
			N:       numOfTxsPerIssuer,
			Timeout: 20 * time.Second,
		},
	)

	if err := orchestrator.Execute(ctx); err != nil {
		r.True(errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled))
	}

	r.Equal(numOfTxs, tracker.GetObservedIssued())
	r.Equal(numOfTxs, tracker.GetObservedConfirmed())
	r.Equal(uint64(0), tracker.GetObservedFailed())
}

var _ = registry.Register(TestsRegistry, "ShortBurst", ShortBurstTest)

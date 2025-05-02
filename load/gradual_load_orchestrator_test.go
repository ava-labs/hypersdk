// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/consts"
)

var (
	_ TxGenerator[ids.ID] = (*mockTxGenerator)(nil)
	_ Issuer[ids.ID]      = (*mockIssuer)(nil)
)

func TestGradualLoadOrchestratorTPS(t *testing.T) {
	tests := []struct {
		name        string
		serverTPS   uint64
		config      GradualLoadOrchestratorConfig
		expectedErr error
	}{
		{
			name:      "orchestrator achieves max TPS",
			serverTPS: consts.MaxUint64,
			config: GradualLoadOrchestratorConfig{
				MaxTPS:           2_000,
				MinTPS:           1_000,
				Step:             1_000,
				TxRateMultiplier: 1.5,
				SustainedTime:    time.Second,
				MaxAttempts:      2,
				Terminate:        true,
			},
		},
		{
			name:      "orchestrator TPS limited by network",
			serverTPS: 1_000,
			config: GradualLoadOrchestratorConfig{
				MaxTPS:           2_000,
				MinTPS:           1_000,
				Step:             1_000,
				TxRateMultiplier: 1.3,
				SustainedTime:    time.Second,
				MaxAttempts:      2,
				Terminate:        true,
			},
			expectedErr: ErrFailedToReachTargetTPS,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tracker, err := NewPrometheusTracker[ids.ID](prometheus.NewRegistry())
			r.NoError(err)

			agents := []Agent[ids.ID, ids.ID]{
				NewAgent(
					&mockTxGenerator{
						generateTxF: func() (ids.ID, error) {
							return ids.GenerateTestID(), nil
						},
					},
					newMockIssuer(tracker, tt.serverTPS),
					&mockListener{},
					tracker,
				),
			}

			orchestrator, err := NewGradualLoadOrchestrator(
				agents,
				logging.NoLog{},
				tt.config,
			)
			r.NoError(err)

			r.ErrorIs(orchestrator.Execute(ctx), tt.expectedErr)

			if tt.expectedErr == nil {
				r.GreaterOrEqual(orchestrator.GetMaxObservedTPS(), tt.config.MaxTPS)
			} else {
				r.Less(orchestrator.GetMaxObservedTPS(), tt.config.MaxTPS)
			}
		})
	}
}

// test that the orchestrator returns early if the txGenerators or the issuers error
func TestGradualLoadOrchestratorExecution(t *testing.T) {
	var (
		errMockTxGenerator = errors.New("mock tx generator error")
		errMockIssuer      = errors.New("mock issuer error")
	)

	tracker, err := NewPrometheusTracker[ids.ID](prometheus.NewRegistry())
	require.NoError(t, err, "creating tracker")

	tests := []struct {
		name        string
		agents      []Agent[ids.ID, ids.ID]
		expectedErr error
	}{
		{
			name: "generator error",
			agents: []Agent[ids.ID, ids.ID]{
				NewAgent[ids.ID, ids.ID](
					&mockTxGenerator{
						generateTxF: func() (ids.ID, error) {
							return ids.Empty, errMockTxGenerator
						},
					},
					&mockIssuer{},
					&mockListener{},
					tracker,
				),
			},
			expectedErr: errMockTxGenerator,
		},
		{
			name: "issuer error",
			agents: []Agent[ids.ID, ids.ID]{
				NewAgent[ids.ID, ids.ID](
					&mockTxGenerator{
						generateTxF: func() (ids.ID, error) {
							return ids.GenerateTestID(), nil
						},
					},
					&mockIssuer{
						issueTxErr: errMockIssuer,
					},
					&mockListener{},
					tracker,
				),
			},
			expectedErr: errMockIssuer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			orchestrator, err := NewGradualLoadOrchestrator(
				tt.agents,
				logging.NoLog{},
				DefaultGradualLoadOrchestratorConfig(),
			)
			r.NoError(err)

			r.ErrorIs(orchestrator.Execute(ctx), tt.expectedErr)
		})
	}
}

type mockTxGenerator struct {
	generateTxF func() (ids.ID, error)
}

func (m *mockTxGenerator) GenerateTx() (ids.ID, error) {
	return m.generateTxF()
}

type mockIssuer struct {
	currTxsIssued uint64
	maxTxs        uint64
	tracker       Tracker[ids.ID]
	issueTxErr    error
}

func newMockIssuer(tracker Tracker[ids.ID], maxTxs uint64) *mockIssuer {
	return &mockIssuer{
		tracker: tracker,
		maxTxs:  maxTxs,
	}
}

// IssueTx immediately issues and confirms a tx
// To simulate TPS, the number of txs IssueTx can issue/confirm is capped by maxTxs
func (m *mockIssuer) IssueTx(_ context.Context, id ids.ID) error {
	if m.issueTxErr != nil {
		return m.issueTxErr
	}

	if m.currTxsIssued >= m.maxTxs {
		return nil
	}

	m.tracker.Issue(id)
	m.tracker.ObserveConfirmed(id)
	m.currTxsIssued++
	return nil
}

type mockListener struct{}

func (*mockListener) Listen(context.Context) error {
	return nil
}

func (*mockListener) RegisterIssued(ids.ID) {}

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
)

var (
	_ TxGenerator[ids.ID] = (*mockTxGenerator)(nil)
	_ Issuer[ids.ID]      = (*mockIssuer)(nil)
	_ Tracker[any]        = (*mockTracker)(nil)
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
			serverTPS: 3_000,
			config: GradualLoadOrchestratorConfig{
				MaxTPS:           2_000,
				MinTPS:           1_000,
				Step:             1_000,
				TxRateMultiplier: 1.3,
				SustainedTime:    time.Second,
				MaxAttempts:      2,
				Terminate:        true,
			},
		},
		{
			name:      "orchestrator TPS limited by network",
			serverTPS: 1_500,
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

			orchestrator, err := NewGradualLoadOrchestrator(
				[]TxGenerator[ids.ID]{&mockTxGenerator{
					generateTxF: func() (ids.ID, error) {
						return ids.GenerateTestID(), nil
					},
				}},
				[]Issuer[ids.ID]{&mockIssuer{}},
				newMockTracker(tt.serverTPS, tt.config.SustainedTime),
				logging.NoLog{},
				tt.config,
			)
			r.NoError(err)

			r.ErrorIs(orchestrator.Execute(ctx), tt.expectedErr, "got %v TPS", orchestrator.GetMaxObservedTPS())

			if tt.expectedErr == nil {
				r.GreaterOrEqual(orchestrator.GetMaxObservedTPS(), tt.config.MaxTPS)
			} else {
				r.Less(orchestrator.GetMaxObservedTPS(), tt.config.MaxTPS)
			}
		})
	}
}

func TestGradualLoadOrchestratorConstructor(t *testing.T) {
	tests := []struct {
		name        string
		issuers     []Issuer[ids.ID]
		generators  []TxGenerator[ids.ID]
		expectedErr error
	}{
		{
			name:       "same number of issuers and generators",
			issuers:    []Issuer[ids.ID]{&mockIssuer{}},
			generators: []TxGenerator[ids.ID]{&mockTxGenerator{}},
		},
		{
			name:        "different number of issuers and generators",
			issuers:     []Issuer[ids.ID]{&mockIssuer{}, &mockIssuer{}},
			generators:  []TxGenerator[ids.ID]{&mockTxGenerator{}},
			expectedErr: ErrMismatchedGeneratorsAndIssuers,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)

			_, err := NewGradualLoadOrchestrator[ids.ID, ids.ID](
				tt.generators,
				tt.issuers,
				nil,
				logging.NoLog{},
				GradualLoadOrchestratorConfig{},
			)

			r.ErrorIs(err, tt.expectedErr)
		})
	}
}

// test that the orchestrator returns early if the txGenerators or the issuers error
func TestGradualLoadOrchestratorExecution(t *testing.T) {
	var (
		errMockTxGenerator = errors.New("mock tx generator error")
		errMockIssuer      = errors.New("mock issuer error")
	)

	tests := []struct {
		name        string
		generators  []TxGenerator[ids.ID]
		issuers     []Issuer[ids.ID]
		expectedErr error
	}{
		{
			name: "generator error",
			generators: []TxGenerator[ids.ID]{
				&mockTxGenerator{
					generateTxF: func() (ids.ID, error) {
						return ids.Empty, errMockTxGenerator
					},
				},
			},
			issuers:     []Issuer[ids.ID]{&mockIssuer{}},
			expectedErr: errMockTxGenerator,
		},
		{
			name: "issuer error",
			generators: []TxGenerator[ids.ID]{
				&mockTxGenerator{
					generateTxF: func() (ids.ID, error) {
						return ids.GenerateTestID(), nil
					},
				},
			},
			issuers: []Issuer[ids.ID]{&mockIssuer{
				issueTxErr: errMockIssuer,
			}},
			expectedErr: errMockIssuer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			tracker, err := NewPrometheusTracker[ids.ID](prometheus.NewRegistry())
			r.NoError(err)

			orchestrator, err := NewGradualLoadOrchestrator(
				tt.generators,
				tt.issuers,
				tracker,
				logging.NoLog{},
				DefaultGradualLoadOrchestratorConfig(),
			)
			r.NoError(err)

			r.ErrorIs(orchestrator.Execute(ctx), tt.expectedErr)
		})
	}
}

type mockTracker struct {
	observedConfirmed uint64
	tps               uint64
	sustainedTime     time.Duration
}

func newMockTracker(tps uint64, sustainedTime time.Duration) *mockTracker {
	return &mockTracker{
		tps:           tps,
		sustainedTime: sustainedTime,
	}
}

// GetObservedConfirmed returns the number of confirmed transactions observed
//
// The gradual load orchestrator calls GetObservedConfirmed every sustainedTime
// seconds to compute the TPS. The mockTracker simulates this by returning a
// counter (observedConfirmed) and incrementing that counter by the number of
// txs it is expected to observe in the next sustainedTime seconds.
func (m *mockTracker) GetObservedConfirmed() uint64 {
	oc := m.observedConfirmed
	m.observedConfirmed += (m.tps * uint64(m.sustainedTime.Seconds()))
	return oc
}

func (*mockTracker) GetObservedFailed() uint64 {
	return 0
}

func (*mockTracker) GetObservedIssued() uint64 {
	return 0
}

func (*mockTracker) Issue(any) {}

func (*mockTracker) ObserveConfirmed(any) {}

func (*mockTracker) ObserveFailed(any) {}

type mockTxGenerator struct {
	generateTxF func() (ids.ID, error)
}

func (m *mockTxGenerator) GenerateTx(context.Context) (ids.ID, error) {
	return m.generateTxF()
}

type mockIssuer struct {
	issueTxErr error
}

func (m *mockIssuer) IssueTx(context.Context, ids.ID) error {
	return m.issueTxErr
}

func (*mockIssuer) Listen(context.Context) error {
	return nil
}

func (*mockIssuer) Stop() {}

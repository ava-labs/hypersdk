// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package load

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

const chanSize = 1_000_000

var (
	_ TxGenerator[ids.ID] = (*mockTxGenerator)(nil)
	_ Issuer[ids.ID]      = (*mockIssuer)(nil)
	_ Issuer[ids.ID]      = (*mockErrIssuer)(nil)
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
			serverTPS: 4_000,
			config: GradualLoadOrchestratorConfig{
				MaxTPS:           4_000,
				MinTPS:           1_000,
				Step:             1_000,
				TxRateMultiplier: 1.3,
				SustainedTime:    3 * time.Second,
				MaxAttempts:      3,
				Terminate:        true,
			},
		},
		{
			name:      "orchestrator TPS limited by network",
			serverTPS: 3_000,
			config: GradualLoadOrchestratorConfig{
				MaxTPS:           4_000,
				MinTPS:           1_000,
				Step:             1_000,
				TxRateMultiplier: 1.3,
				SustainedTime:    3 * time.Second,
				MaxAttempts:      3,
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

			server := newMockServer(tt.serverTPS)
			issuer := newMockIssuer("issuer1", server, tracker)

			// Start server
			go server.run(ctx)

			orchestrator, err := NewGradualLoadOrchestrator(
				[]TxGenerator[ids.ID]{&mockTxGenerator{
					generateTxF: func() (ids.ID, error) {
						return ids.GenerateTestID(), nil
					},
				}},
				[]Issuer[ids.ID]{issuer},
				tracker,
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

func TestGradualLoadOrchestratorInitialization(t *testing.T) {
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
			issuers: []Issuer[ids.ID]{&mockErrIssuer{
				err: errMockIssuer,
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
				GradualLoadOrchestratorConfig{
					MaxTPS:           1,
					MinTPS:           1,
					Step:             1,
					TxRateMultiplier: 1,
					SustainedTime:    time.Second,
					MaxAttempts:      1,
					Terminate:        true,
				},
			)
			r.NoError(err)

			r.ErrorIs(orchestrator.Execute(ctx), tt.expectedErr)
		})
	}
}

type mockTxGenerator struct {
	generateTxF func() (ids.ID, error)
}

func (m *mockTxGenerator) GenerateTx(context.Context) (ids.ID, error) {
	return m.generateTxF()
}

type mockIssuer struct {
	name     string
	server   *mockServer
	incoming <-chan ids.ID
	tracker  Tracker[ids.ID]
}

// newMockIssuer creates an issuer that is already connected to the server
func newMockIssuer(name string, server *mockServer, tracker Tracker[ids.ID]) *mockIssuer {
	return &mockIssuer{
		name:     name,
		server:   server,
		tracker:  tracker,
		incoming: server.connect(name),
	}
}

func (m *mockIssuer) IssueTx(_ context.Context, tx ids.ID) error {
	m.server.send(tx, m.name)
	m.tracker.Issue(tx)
	return nil
}

func (m *mockIssuer) Listen(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case tx := <-m.incoming:
			m.tracker.ObserveConfirmed(tx)
		}
	}
}

func (*mockIssuer) Stop() {}

type mockErrIssuer struct {
	mockIssuer
	err error
}

func (m *mockErrIssuer) IssueTx(context.Context, ids.ID) error {
	return m.err
}

// mockServer simulates a network that confirms transactions at a fixed rate
type mockServer struct {
	// number of transactions to "confirm" each second
	tps uint64

	sync.RWMutex
	// maps txs to sender
	outstandingTxs map[ids.ID]string
	// connections to clients
	conns map[string]chan ids.ID
	// txs from clients
	incoming chan ids.ID
}

func newMockServer(tps uint64) *mockServer {
	return &mockServer{
		outstandingTxs: make(map[ids.ID]string),
		conns:          make(map[string]chan ids.ID),
		incoming:       make(chan ids.ID, chanSize),
		tps:            tps,
	}
}

// connect a client to the server
// returns a channel that the server will use to send confirmed txs to the
// client
// the client should read from this channel to hear about confirmed txs
func (m *mockServer) connect(client string) <-chan ids.ID {
	m.Lock()
	defer m.Unlock()

	out := make(chan ids.ID, chanSize)
	m.conns[client] = out
	return out
}

// send a tx to the server
func (m *mockServer) send(tx ids.ID, client string) {
	m.Lock()
	defer m.Unlock()

	m.outstandingTxs[tx] = client
	m.incoming <- tx
}

// run simulates a network running at maxTPS
// each second, the server will send maxTPS txs to the clients
// if there are not enough txs to send, the server will sleep until the next second
func (m *mockServer) run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)

	go func() {
		<-ctx.Done()
		ticker.Stop()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for range m.tps {
				select {
				case tx := <-m.incoming:
					m.notify(tx)
				default:
				}
			}
		}
	}
}

// notify the tx sender of the tx's status
func (m *mockServer) notify(tx ids.ID) {
	m.Lock()
	defer m.Unlock()

	sender := m.outstandingTxs[tx]
	m.conns[sender] <- tx
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/internal/validitywindow/validitywindowtest"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/metadata"
)

var (
	_ chain.AuthVM = (*mockAuthVM)(nil)

	heightKey    = string(chain.HeightKey([]byte{0}))
	timestampKey = string(chain.TimestampKey([]byte{1}))

	errMockWorker                       = errors.New("mock worker error")
	errMockWorkerJob                    = errors.New("mock worker job error")
	errMockVerifyExpiryReplayProtection = errors.New("mock validity window error")
)

type createBlock func(parentRoot ids.ID) (*chain.StatelessBlock, error)

type mockAuthVM struct{}

func (m *mockAuthVM) GetAuthBatchVerifier(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool) {
	return nil, false
}

func (m *mockAuthVM) Logger() logging.Logger {
	panic("unimplemented")
}

func TestProcessorExecute(t *testing.T) {
	testRules := genesis.NewDefaultRules()
	testRuleFactory := genesis.ImmutableRuleFactory{Rules: testRules}
	createValidBlock := func(root ids.ID) (*chain.StatelessBlock, error) {
		return chain.NewStatelessBlock(
			ids.Empty,
			testRules.GetMinEmptyBlockGap(),
			1,
			nil,
			root,
		)
	}

	tests := []struct {
		name           string
		validityWindow chain.ValidityWindow
		isNormalOp     bool
		createBlock    createBlock
		state          map[string][]byte
		workers        workers.Workers
		expectedErr    error
	}{
		{
			name: "valid test case",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
				feeKey:       {},
			},
			createBlock: createValidBlock,
		},
		{
			name: "block timestamp too late",
			createBlock: func(root ids.ID) (*chain.StatelessBlock, error) {
				return chain.NewStatelessBlock(
					ids.Empty,
					time.Now().Add(chain.FutureBound).UnixMilli()+int64(time.Second),
					0,
					nil,
					root,
				)
			},
			expectedErr: chain.ErrTimestampTooLate,
		},
		{
			name:        "verify signatures fails",
			createBlock: createValidBlock,
			workers: &workers.MockWorkers{
				OnNewJob: func(_ int) (workers.Job, error) {
					return nil, errMockWorker
				},
			},
			expectedErr: errMockWorker,
		},
		{
			name:        "failed to get parent height",
			createBlock: createValidBlock,
			expectedErr: chain.ErrFailedToFetchParentHeight,
		},
		{
			name: "failed to parse parent height",
			state: map[string][]byte{
				heightKey: {},
			},
			createBlock: createValidBlock,
			expectedErr: chain.ErrFailedToParseParentHeight,
		},
		{
			name: "block height is not one more than parent height (2 != 0 + 1)",
			state: map[string][]byte{
				heightKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			createBlock: func(parentRoot ids.ID) (*chain.StatelessBlock, error) {
				return chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					2,
					nil,
					parentRoot,
				)
			},
			expectedErr: chain.ErrInvalidBlockHeight,
		},
		{
			name: "failed to get timestamp",
			state: map[string][]byte{
				heightKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			createBlock: createValidBlock,
			expectedErr: chain.ErrFailedToFetchParentTimestamp,
		},
		{
			name: "failed to parse timestamp",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: {},
			},
			createBlock: createValidBlock,
			expectedErr: chain.ErrFailedToParseParentTimestamp,
		},
		{
			name: "non-empty block timestamp less than parent timestamp with gap",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			createBlock: func(parentRoot ids.ID) (*chain.StatelessBlock, error) {
				return chain.NewStatelessBlock(
					ids.Empty,
					0,
					1,
					[]*chain.Transaction{
						{
							TransactionData: chain.TransactionData{
								Base: &chain.Base{},
							},
							Auth: &mockAuth{
								typeID: 1,
							},
						},
					},
					parentRoot,
				)
			},
			expectedErr: chain.ErrTimestampTooEarly,
		},
		{
			name: "empty block timestamp less than parent timestamp with gap",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			createBlock: func(parentRoot ids.ID) (*chain.StatelessBlock, error) {
				return chain.NewStatelessBlock(
					ids.Empty,
					0,
					1,
					nil,
					parentRoot,
				)
			},
			expectedErr: chain.ErrTimestampTooEarly,
		},
		{
			name: "failed to get fee",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			createBlock: createValidBlock,
			expectedErr: chain.ErrFailedToFetchParentFee,
		},
		{
			name: "failed VerifyExpiryReplayProtection",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
				feeKey:       {},
			},
			createBlock: createValidBlock,
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{
				OnVerifyExpiryReplayProtection: func(_ context.Context, _ validitywindow.ExecutionBlock[*chain.Transaction]) error {
					return errMockVerifyExpiryReplayProtection
				},
			},
			isNormalOp:  true,
			expectedErr: errMockVerifyExpiryReplayProtection,
		},
		{
			name: "failed to execute txs",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
				feeKey:       {},
			},
			createBlock: func(parentRoot ids.ID) (*chain.StatelessBlock, error) {
				return chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					[]*chain.Transaction{
						{
							TransactionData: chain.TransactionData{
								Base: &chain.Base{},
								Actions: []chain.Action{
									&mockAction{
										stateKeys: state.Keys{
											"": state.None,
										},
									},
								},
							},
							Auth: &mockAuth{
								typeID: 1,
							},
						},
					},
					parentRoot,
				)
			},
			expectedErr: chain.ErrInvalidKeyValue,
		},
		{
			name: "state root mismatch",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
				feeKey:       {},
			},
			createBlock: func(parentRoot ids.ID) (*chain.StatelessBlock, error) {
				return chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					ids.GenerateTestID(),
				)
			},
			expectedErr: chain.ErrStateRootMismatch,
		},
		{
			name: "failed to verify signatures",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
				feeKey:       {},
			},
			createBlock: createValidBlock,
			workers: &workers.MockWorkers{
				OnNewJob: func(_ int) (workers.Job, error) {
					return &workers.MockJob{
						OnWaitError: errMockWorkerJob,
					}, nil
				},
			},
			expectedErr: errMockWorkerJob,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()

			if tt.workers == nil {
				tt.workers = &workers.MockWorkers{
					OnNewJob: func(_ int) (workers.Job, error) {
						return &workers.MockJob{}, nil
					},
				}
			}

			if tt.validityWindow == nil {
				tt.validityWindow = &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{}
			}

			metrics, err := chain.NewMetrics(prometheus.NewRegistry())
			r.NoError(err)

			processor := chain.NewProcessor(
				trace.Noop,
				&logging.NoLog{},
				&testRuleFactory,
				tt.workers,
				&mockAuthVM{},
				metadata.NewDefaultManager(),
				nil,
				tt.validityWindow,
				metrics,
				chain.Config{},
			)

			db, err := merkledb.New(
				ctx,
				memdb.New(),
				merkledb.Config{
					BranchFactor: merkledb.BranchFactor16,
					Tracer:       trace.Noop,
				},
			)
			r.NoError(err)

			for k, v := range tt.state {
				r.NoError(db.Put([]byte(k), v))
			}
			r.NoError(db.CommitToDB(ctx))

			root, err := db.GetMerkleRoot(ctx)
			r.NoError(err)

			block, err := tt.createBlock(root)
			r.NoError(err)

			_, err = processor.Execute(ctx, db, chain.NewExecutionBlock(block), tt.isNormalOp)
			r.ErrorIs(err, tt.expectedErr)
		})
	}
}

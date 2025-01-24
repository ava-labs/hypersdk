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
	heightKey    = string(chain.HeightKey([]byte{0}))
	timestampKey = string(chain.TimestampKey([]byte{1}))

	errMockWorker                       = errors.New("mock worker error")
	errMockWorkerJob                    = errors.New("mock worker job error")
	errMockVerifyExpiryReplayProtection = errors.New("mock validity window error")
)

func TestProcessorExecute(t *testing.T) {
	testRules := genesis.NewDefaultRules()
	testRuleFactory := genesis.ImmutableRuleFactory{Rules: testRules}
	invalidStateRoot := ids.ID{1}
	validBlock := &chain.ExecutionBlock{
		StatelessBlock: &chain.StatelessBlock{
			Hght:   1,
			Tmstmp: testRules.GetMinEmptyBlockGap(),
		},
	}

	tests := []struct {
		name           string
		block          *chain.ExecutionBlock
		validityWindow chain.ValidityWindow
		includeRoot    bool
		isNormalOp     bool
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
			block:       validBlock,
			includeRoot: true,
		},
		{
			name: "block timestamp too late",
			block: &chain.ExecutionBlock{
				StatelessBlock: &chain.StatelessBlock{
					Tmstmp: time.Now().Add(chain.FutureBound).UnixMilli() + int64(time.Second),
				},
			},
			expectedErr: chain.ErrTimestampTooLate,
		},
		{
			name:  "verify signatures fails",
			block: validBlock,
			workers: &workers.MockWorkers{
				OnNewJob: func(_ int) (workers.Job, error) {
					return nil, errMockWorker
				},
			},
			expectedErr: errMockWorker,
		},
		{
			name:        "failed to get parent height",
			block:       validBlock,
			expectedErr: chain.ErrFailedToFetchParentHeight,
		},
		{
			name: "failed to parse parent height",
			state: map[string][]byte{
				heightKey: {},
			},
			block:       validBlock,
			expectedErr: chain.ErrFailedToParseParentHeight,
		},
		{
			name: "block height is not one more than parent height (2 != 0 + 1)",
			state: map[string][]byte{
				heightKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			block: &chain.ExecutionBlock{
				StatelessBlock: &chain.StatelessBlock{
					Hght: 2,
				},
			},
			expectedErr: chain.ErrInvalidBlockHeight,
		},
		{
			name: "failed to get timestamp",
			state: map[string][]byte{
				heightKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			block:       validBlock,
			expectedErr: chain.ErrFailedToFetchParentTimestamp,
		},
		{
			name: "failed to parse timestamp",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: {},
			},
			block:       validBlock,
			expectedErr: chain.ErrFailedToParseParentTimestamp,
		},
		{
			name: "non-empty block timestamp less than parent timestamp with gap",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			block: &chain.ExecutionBlock{
				StatelessBlock: &chain.StatelessBlock{
					Hght: 1,
					Txs: []*chain.Transaction{
						{
							TransactionData: chain.TransactionData{
								Base: &chain.Base{},
							},
							Auth: &mockAuth{
								typeID: 1,
							},
						},
					},
				},
			},
			expectedErr: chain.ErrTimestampTooEarly,
		},
		{
			name: "empty block timestamp less than parent timestamp with gap",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			block: &chain.ExecutionBlock{
				StatelessBlock: &chain.StatelessBlock{
					Hght: 1,
				},
			},
			expectedErr: chain.ErrTimestampTooEarly,
		},
		{
			name: "failed to get fee",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			block:       validBlock,
			expectedErr: chain.ErrFailedToFetchParentFee,
		},
		{
			name: "failed VerifyExpiryReplayProtection",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
				feeKey:       {},
			},
			block: validBlock,
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
			block: &chain.ExecutionBlock{
				StatelessBlock: &chain.StatelessBlock{
					Hght:   1,
					Tmstmp: testRules.GetMinEmptyBlockGap(),
					Txs: []*chain.Transaction{
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
				},
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
			block: &chain.ExecutionBlock{
				StatelessBlock: &chain.StatelessBlock{
					Hght:      1,
					Tmstmp:    testRules.GetMinEmptyBlockGap(),
					StateRoot: invalidStateRoot,
				},
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
			block: validBlock,
			workers: &workers.MockWorkers{
				OnNewJob: func(_ int) (workers.Job, error) {
					return &workers.MockJob{
						OnWaitError: errMockWorkerJob,
					}, nil
				},
			},
			includeRoot: true,
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
				nil,
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

			if tt.includeRoot {
				root, err := db.GetMerkleRoot(ctx)
				r.NoError(err)
				tt.block.StateRoot = root
			}

			_, err = processor.Execute(ctx, db, tt.block, tt.isNormalOp)
			r.ErrorIs(err, tt.expectedErr)
		})
	}
}

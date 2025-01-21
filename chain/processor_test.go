// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/executor"
	"github.com/ava-labs/hypersdk/internal/logging"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/metadata"
)

var (
	heightKey    = string(chain.HeightKey([]byte{0}))
	timestampKey = string(chain.TimestampKey([]byte{1}))
	feeKey       = string(chain.FeeKey([]byte{2}))
)

var (
	errVerifyExpiryReplayProtection = errors.New("mock chain index error")

	_ executor.Metrics     = (*mockExecutorMetrics)(nil)
	_ chain.ValidityWindow = (*mockValidityWindow)(nil)
	_ chain.Action         = (*mockAction1)(nil)
)

type mockValidityWindow struct {
	verifyExpiryReplayProtectionErr error
}

func (m *mockValidityWindow) IsRepeat(ctx context.Context, parentBlk validitywindow.ExecutionBlock[*chain.Transaction], txs []*chain.Transaction, oldestAllowed int64) (set.Bits, error) {
	panic("unimplemented")
}

func (m *mockValidityWindow) VerifyExpiryReplayProtection(ctx context.Context, blk validitywindow.ExecutionBlock[*chain.Transaction], oldestAllowed int64) error {
	return m.verifyExpiryReplayProtectionErr
}

func (m *mockValidityWindow) Accept(blk validitywindow.ExecutionBlock[*chain.Transaction]) {
	panic("unimplemented")
}

type mockExecutorMetrics struct{}

func (m *mockExecutorMetrics) RecordBlocked() {}

func (m *mockExecutorMetrics) RecordExecutable() {}

type mockAction1 struct {
	mockAction
	stateKeys state.Keys
}

func (m *mockAction1) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return m.stateKeys
}

func TestProcessorExecute(t *testing.T) {
	testRules := genesis.NewDefaultRules()
	testRuleFactory := genesis.ImmutableRuleFactory{
		Rules: testRules,
	}

	tests := []struct {
		name           string
		tx             *chain.Transaction
		block          *chain.ExecutionBlock
		validityWindow chain.ValidityWindow
		includeRoot    bool
		state          map[string][]byte
		workersFail    bool
		expectedErr    error
	}{
		{
			name: "valid test case",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
				feeKey:       {},
			},
			block: &chain.ExecutionBlock{
				StatelessBlock: &chain.StatelessBlock{
					Hght:   1,
					Tmstmp: testRules.GetMinEmptyBlockGap(),
				},
			},
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
			name: "async verify fails",
			block: &chain.ExecutionBlock{
				StatelessBlock: &chain.StatelessBlock{},
			},
			workersFail: true,
			expectedErr: workers.ErrShutdown,
		},
		{
			name: "failed to get parent height",
			block: &chain.ExecutionBlock{
				StatelessBlock: &chain.StatelessBlock{},
			},
			expectedErr: database.ErrNotFound,
		},
		{
			name: "failed to parse parent height",
			state: map[string][]byte{
				heightKey: {},
			},
			block: &chain.ExecutionBlock{
				StatelessBlock: &chain.StatelessBlock{},
			},
			expectedErr: chain.ErrParsingInvalidUint64,
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
			block: &chain.ExecutionBlock{
				StatelessBlock: &chain.StatelessBlock{
					Hght: 1,
				},
			},
			expectedErr: database.ErrNotFound,
		},
		{
			name: "failed to parse timestamp",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: {},
			},
			block: &chain.ExecutionBlock{
				StatelessBlock: &chain.StatelessBlock{
					Hght: 1,
				},
			},
			expectedErr: chain.ErrParsingInvalidUint64,
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
			name: "failed VerifyExpiryReplayProtection",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			block: &chain.ExecutionBlock{
				StatelessBlock: &chain.StatelessBlock{
					Hght:   1,
					Tmstmp: testRules.GetMinEmptyBlockGap(),
				},
			},
			validityWindow: &mockValidityWindow{
				verifyExpiryReplayProtectionErr: errVerifyExpiryReplayProtection,
			},
			expectedErr: errVerifyExpiryReplayProtection,
		},
		{
			name: "failed to get fee",
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			block: &chain.ExecutionBlock{
				StatelessBlock: &chain.StatelessBlock{
					Hght:   1,
					Tmstmp: testRules.GetMinEmptyBlockGap(),
				},
			},
			expectedErr: database.ErrNotFound,
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
									&mockAction1{
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
					Hght:   1,
					Tmstmp: testRules.GetMinEmptyBlockGap(),
				},
			},
			expectedErr: chain.ErrStateRootMismatch,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()

			// TODO: explain values
			workers := workers.NewParallel(1, 1)
			if tt.workersFail {
				workers.Stop()
			}
			if tt.validityWindow == nil {
				tt.validityWindow = &mockValidityWindow{}
			}

			testChain, err := chain.NewChain(
				trace.Noop,
				prometheus.NewRegistry(),
				nil,
				nil,
				&logging.Noop{},
				&testRuleFactory,
				metadata.NewDefaultManager(),
				nil,
				workers,
				nil,
				tt.validityWindow,
				chain.Config{},
			)
			r.NoError(err)

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

			_, _, err = testChain.Execute(ctx, db, tt.block)
			r.ErrorIs(err, tt.expectedErr)
		})
	}
}

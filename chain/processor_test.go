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

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/internal/validitywindow/validitywindowtest"
	"github.com/ava-labs/hypersdk/internal/workers"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/metadata"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	_ chain.AuthVM = (*mockAuthVM)(nil)

	heightKey    = string(chain.HeightKey([]byte{0}))
	timestampKey = string(chain.TimestampKey([]byte{1}))

	errMockVerifyExpiryReplayProtection = errors.New("mock validity window error")
)

type createBlock func(parentRoot ids.ID) *chain.StatelessBlock

type mockAuthVM struct{}

func (*mockAuthVM) GetAuthBatchVerifier(_ uint8, _ int, _ int) (chain.AuthBatchVerifier, bool) {
	return nil, false
}

func (*mockAuthVM) Logger() logging.Logger {
	panic("unimplemented")
}

func TestProcessorExecute(t *testing.T) {
	testRules := genesis.NewDefaultRules()
	testRuleFactory := genesis.ImmutableRuleFactory{Rules: testRules}
	createValidBlock := func(root ids.ID) *chain.StatelessBlock {
		block, err := chain.NewStatelessBlock(
			ids.Empty,
			time.Now().UnixMilli(),
			1,
			nil,
			root,
		)
		require.NoError(t, err)
		return block
	}

	tests := []struct {
		name           string
		validityWindow chain.ValidityWindow
		workers        workers.Workers
		isNormalOp     bool
		state          map[string][]byte
		createBlock    createBlock
		expectedErr    error
	}{
		{
			name:           "valid test case",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
				feeKey:       {},
			},
			createBlock: createValidBlock,
		},
		{
			name:           "block timestamp too late",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			createBlock: func(root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					time.Now().Add(chain.FutureBound).UnixMilli()+int64(time.Second),
					0,
					nil,
					root,
				)
				require.NoError(t, err)
				return block
			},
			expectedErr: chain.ErrTimestampTooLate,
		},
		{
			name:           "verify signatures fails",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers: func() workers.Workers {
				w := workers.NewParallel(0, 0)
				w.Stop()
				return w
			}(),
			createBlock: createValidBlock,
			expectedErr: workers.ErrShutdown,
		},
		{
			name:           "failed to get parent height",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			createBlock:    createValidBlock,
			expectedErr:    chain.ErrFailedToFetchParentHeight,
		},
		{
			name:           "failed to parse parent height",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			state: map[string][]byte{
				heightKey: {},
			},
			createBlock: createValidBlock,
			expectedErr: chain.ErrFailedToParseParentHeight,
		},
		{
			name:           "block height is not one more than parent height (2 != 0 + 1)",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			state: map[string][]byte{
				heightKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			createBlock: func(parentRoot ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					time.Now().UnixMilli(),
					2,
					nil,
					parentRoot,
				)
				require.NoError(t, err)
				return block
			},
			expectedErr: chain.ErrInvalidBlockHeight,
		},
		{
			name:           "failed to get timestamp",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			state: map[string][]byte{
				heightKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			createBlock: createValidBlock,
			expectedErr: chain.ErrFailedToFetchParentTimestamp,
		},
		{
			name:           "failed to parse timestamp",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: {},
			},
			createBlock: createValidBlock,
			expectedErr: chain.ErrFailedToParseParentTimestamp,
		},
		{
			name:           "non-empty block timestamp less than parent timestamp with gap",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			createBlock: func(parentRoot ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					0,
					1,
					[]*chain.Transaction{
						func() *chain.Transaction {
							r := require.New(t)
							tx, err := chain.NewTransaction(
								&chain.Base{},
								[]chain.Action{},
								&mockAuth{
									typeID: 1,
								},
							)
							r.NoError(err)
							return tx
						}(),
					},
					parentRoot,
				)
				require.NoError(t, err)
				return block
			},
			expectedErr: chain.ErrTimestampTooEarly,
		},
		{
			name:           "empty block timestamp less than parent timestamp with gap",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			createBlock: func(parentRoot ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					0,
					1,
					nil,
					parentRoot,
				)
				require.NoError(t, err)
				return block
			},
			expectedErr: chain.ErrTimestampTooEarly,
		},
		{
			name:           "failed to get fee",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
			},
			createBlock: createValidBlock,
			expectedErr: chain.ErrFailedToFetchParentFee,
		},
		{
			name: "fails replay protection",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{
				OnVerifyExpiryReplayProtection: func(_ context.Context, _ validitywindow.ExecutionBlock[*chain.Transaction]) error {
					return errMockVerifyExpiryReplayProtection
				},
			},
			workers:    workers.NewSerial(),
			isNormalOp: true,
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
				feeKey:       {},
			},
			createBlock: createValidBlock,
			expectedErr: errMockVerifyExpiryReplayProtection,
		},
		{
			name:           "failed to execute txs",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
				feeKey:       {},
			},
			createBlock: func(parentRoot ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					time.Now().UnixMilli(),
					1,
					[]*chain.Transaction{
						func() *chain.Transaction {
							r := require.New(t)
							tx, err := chain.NewTransaction(
								&chain.Base{},
								[]chain.Action{
									&mockAction{
										stateKeys: state.Keys{
											"": state.None,
										},
									},
								},
								&mockAuth{
									typeID: 1,
								},
							)
							r.NoError(err)
							return tx
						}(),
					},
					parentRoot,
				)
				require.NoError(t, err)
				return block
			},
			expectedErr: chain.ErrInvalidKeyValue,
		},
		{
			name:           "state root mismatch",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
				feeKey:       {},
			},
			createBlock: func(_ ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					time.Now().UnixMilli(),
					1,
					nil,
					ids.GenerateTestID(),
				)
				require.NoError(t, err)
				return block
			},
			expectedErr: chain.ErrStateRootMismatch,
		},
		{
			name:           "failed to verify signatures",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			state: map[string][]byte{
				heightKey:    binary.BigEndian.AppendUint64(nil, 0),
				timestampKey: binary.BigEndian.AppendUint64(nil, 0),
				feeKey:       {},
			},
			createBlock: func(parentRoot ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					time.Now().UnixMilli(),
					1,
					[]*chain.Transaction{
						func() *chain.Transaction {
							r := require.New(t)

							p, err := ed25519.GeneratePrivateKey()
							r.NoError(err)

							tx, err := chain.NewTransaction(
								&chain.Base{
									Timestamp: utils.UnixRMilli(
										time.Now().UnixMilli(),
										testRules.GetValidityWindow(),
									),
								},
								[]chain.Action{},
								&auth.ED25519{
									Signer: p.PublicKey(),
								},
							)
							r.NoError(err)
							return tx
						}(),
					},
					parentRoot,
				)
				require.NoError(t, err)
				return block
			},
			expectedErr: crypto.ErrInvalidSignature,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()

			metrics, err := chain.NewMetrics(prometheus.NewRegistry())
			r.NoError(err)

			processor := chain.NewProcessor(
				trace.Noop,
				&logging.NoLog{},
				&testRuleFactory,
				tt.workers,
				&mockAuthVM{},
				metadata.NewDefaultManager(),
				&mockBalanceHandler{},
				tt.validityWindow,
				metrics,
				chain.NewDefaultConfig(),
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

			_, err = processor.Execute(
				ctx,
				db,
				chain.NewExecutionBlock(tt.createBlock(root)),
				tt.isNormalOp,
			)
			r.ErrorIs(err, tt.expectedErr)
		})
	}
}

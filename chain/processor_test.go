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

func TestProcessorExecute(t *testing.T) {
	currentTime := time.UnixMilli(genesis.NewDefaultRules().GetMinEmptyBlockGap())
	validBlockF := func(root ids.ID) *chain.StatelessBlock {
		block, err := chain.NewStatelessBlock(
			ids.Empty,
			currentTime.UnixMilli(),
			1,
			nil,
			root,
		)
		require.NoError(t, err)
		return block
	}
	dbF := func() merkledb.MerkleDB {
		db, err := merkledb.New(
			context.Background(),
			memdb.New(),
			merkledb.Config{
				BranchFactor: merkledb.BranchFactor16,
				Tracer:       trace.Noop,
			},
		)
		require.NoError(t, err)
		return db
	}

	tests := []struct {
		name           string
		validityWindow chain.ValidityWindow
		workers        workers.Workers
		isNormalOp     bool
		db             merkledb.MerkleDB
		newBlockF      func(ids.ID) *chain.StatelessBlock
		expectedErr    error
	}{
		{
			name:           "valid test case",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			db: func() merkledb.MerkleDB {
				db := dbF()
				require.NoError(t, db.Put([]byte(heightKey), binary.BigEndian.AppendUint64(nil, 0)))
				require.NoError(t, db.Put([]byte(timestampKey), binary.BigEndian.AppendUint64(nil, 0)))
				require.NoError(t, db.Put([]byte(feeKey), []byte{}))
				return db
			}(),
			newBlockF: validBlockF,
		},
		{
			name:           "block timestamp too late",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			newBlockF: func(root ids.ID) *chain.StatelessBlock {
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
			db:          dbF(),
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
			db:          dbF(),
			newBlockF:   validBlockF,
			expectedErr: workers.ErrShutdown,
		},
		{
			name:           "failed to get parent height",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			db:             dbF(),
			newBlockF:      validBlockF,
			expectedErr:    chain.ErrFailedToFetchParentHeight,
		},
		{
			name:           "failed to parse parent height",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			db: func() merkledb.MerkleDB {
				db := dbF()
				require.NoError(t, db.Put([]byte(heightKey), []byte{}))
				return db
			}(),
			newBlockF:   validBlockF,
			expectedErr: chain.ErrFailedToParseParentHeight,
		},
		{
			name:           "block height is not one more than parent height",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			db: func() merkledb.MerkleDB {
				db := dbF()
				require.NoError(t, db.Put([]byte(heightKey), binary.BigEndian.AppendUint64(nil, 0)))
				return db
			}(),
			newBlockF: func(parentRoot ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					currentTime.UnixMilli(),
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
			db: func() merkledb.MerkleDB {
				db := dbF()
				require.NoError(t, db.Put([]byte(heightKey), binary.BigEndian.AppendUint64(nil, 0)))
				return db
			}(),
			newBlockF:   validBlockF,
			expectedErr: chain.ErrFailedToFetchParentTimestamp,
		},
		{
			name:           "failed to parse timestamp",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			db: func() merkledb.MerkleDB {
				db := dbF()
				require.NoError(t, db.Put([]byte(heightKey), binary.BigEndian.AppendUint64(nil, 0)))
				require.NoError(t, db.Put([]byte(timestampKey), []byte{}))
				return db
			}(),
			newBlockF:   validBlockF,
			expectedErr: chain.ErrFailedToParseParentTimestamp,
		},
		{
			name:           "non-empty block timestamp less than parent timestamp with gap",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			db: func() merkledb.MerkleDB {
				db := dbF()
				require.NoError(t, db.Put([]byte(heightKey), binary.BigEndian.AppendUint64(nil, 0)))
				require.NoError(t, db.Put([]byte(timestampKey), binary.BigEndian.AppendUint64(nil, 0)))
				return db
			}(),
			newBlockF: func(parentRoot ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					0,
					1,
					[]*chain.Transaction{
						func() *chain.Transaction {
							tx, err := chain.NewTransaction(
								&chain.Base{},
								[]chain.Action{},
								&mockAuth{
									typeID: 1,
								},
							)
							require.NoError(t, err)
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
			db: func() merkledb.MerkleDB {
				db := dbF()
				require.NoError(t, db.Put([]byte(heightKey), binary.BigEndian.AppendUint64(nil, 0)))
				require.NoError(t, db.Put([]byte(timestampKey), binary.BigEndian.AppendUint64(nil, 0)))
				return db
			}(),
			newBlockF: func(parentRoot ids.ID) *chain.StatelessBlock {
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
			db: func() merkledb.MerkleDB {
				db := dbF()
				require.NoError(t, db.Put([]byte(heightKey), binary.BigEndian.AppendUint64(nil, 0)))
				require.NoError(t, db.Put([]byte(timestampKey), binary.BigEndian.AppendUint64(nil, 0)))
				return db
			}(),
			newBlockF:   validBlockF,
			expectedErr: chain.ErrFailedToFetchParentFee,
		},
		{
			name: "fails replay protection",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{
				OnVerifyExpiryReplayProtection: func(context.Context, validitywindow.ExecutionBlock[*chain.Transaction]) error {
					return errMockVerifyExpiryReplayProtection
				},
			},
			workers:    workers.NewSerial(),
			isNormalOp: true,
			db: func() merkledb.MerkleDB {
				db := dbF()
				require.NoError(t, db.Put([]byte(heightKey), binary.BigEndian.AppendUint64(nil, 0)))
				require.NoError(t, db.Put([]byte(timestampKey), binary.BigEndian.AppendUint64(nil, 0)))
				require.NoError(t, db.Put([]byte(feeKey), []byte{}))
				return db
			}(),
			newBlockF:   validBlockF,
			expectedErr: errMockVerifyExpiryReplayProtection,
		},
		{
			name:           "failed to execute txs",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			workers:        workers.NewSerial(),
			db: func() merkledb.MerkleDB {
				db := dbF()
				require.NoError(t, db.Put([]byte(heightKey), binary.BigEndian.AppendUint64(nil, 0)))
				require.NoError(t, db.Put([]byte(timestampKey), binary.BigEndian.AppendUint64(nil, 0)))
				require.NoError(t, db.Put([]byte(feeKey), []byte{}))
				return db
			}(),
			newBlockF: func(parentRoot ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					currentTime.UnixMilli(),
					1,
					[]*chain.Transaction{
						func() *chain.Transaction {
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
							require.NoError(t, err)
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
			db: func() merkledb.MerkleDB {
				db := dbF()
				require.NoError(t, db.Put([]byte(heightKey), binary.BigEndian.AppendUint64(nil, 0)))
				require.NoError(t, db.Put([]byte(timestampKey), binary.BigEndian.AppendUint64(nil, 0)))
				require.NoError(t, db.Put([]byte(feeKey), []byte{}))
				return db
			}(),
			newBlockF: func(ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					currentTime.UnixMilli(),
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
			db: func() merkledb.MerkleDB {
				db := dbF()
				require.NoError(t, db.Put([]byte(heightKey), binary.BigEndian.AppendUint64(nil, 0)))
				require.NoError(t, db.Put([]byte(timestampKey), binary.BigEndian.AppendUint64(nil, 0)))
				require.NoError(t, db.Put([]byte(feeKey), []byte{}))
				return db
			}(),
			newBlockF: func(parentRoot ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					currentTime.UnixMilli(),
					1,
					[]*chain.Transaction{
						func() *chain.Transaction {
							p, err := ed25519.GeneratePrivateKey()
							require.NoError(t, err)

							testRules := genesis.NewDefaultRules()
							tx, err := chain.NewTransaction(
								&chain.Base{
									Timestamp: utils.UnixRMilli(
										currentTime.UnixMilli(),
										testRules.GetValidityWindow(),
									),
								},
								[]chain.Action{},
								&auth.ED25519{
									Signer: p.PublicKey(),
								},
							)
							require.NoError(t, err)
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
				&genesis.ImmutableRuleFactory{Rules: genesis.NewDefaultRules()},
				tt.workers,
				&mockAuthVM{},
				metadata.NewDefaultManager(),
				&mockBalanceHandler{},
				tt.validityWindow,
				metrics,
				chain.NewDefaultConfig(),
			)

			root, err := tt.db.GetMerkleRoot(ctx)
			r.NoError(err)

			_, err = processor.Execute(
				ctx,
				tt.db,
				chain.NewExecutionBlock(tt.newBlockF(root)),
				tt.isNormalOp,
			)
			r.ErrorIs(err, tt.expectedErr)
		})
	}
}

type mockAuthVM struct{}

func (*mockAuthVM) GetAuthBatchVerifier(uint8, int, int) (chain.AuthBatchVerifier, bool) {
	return nil, false
}

func (*mockAuthVM) Logger() logging.Logger {
	panic("unimplemented")
}

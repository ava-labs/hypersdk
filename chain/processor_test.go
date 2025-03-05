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
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
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

	errMockVerifyExpiryReplayProtection = errors.New("mock validity window error")
)

func TestProcessorExecute(t *testing.T) {
	testRules := genesis.NewDefaultRules()

	testMetadataManager := metadata.NewDefaultManager()
	feeKey := string(chain.FeeKey(testMetadataManager.FeePrefix()))
	heightKey := string(chain.HeightKey(testMetadataManager.HeightPrefix()))
	timestampKey := string(chain.TimestampKey(testMetadataManager.TimestampPrefix()))

	tests := []struct {
		name           string
		validityWindow chain.ValidityWindow
		isNormalOp     bool
		newViewF       func(*require.Assertions) merkledb.View
		newBlockF      func(*require.Assertions, ids.ID) *chain.StatelessBlock
		expectedErr    error
	}{
		{
			name:           "valid test case",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
		},
		{
			name:           "block timestamp too late",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					time.Now().Add(2*chain.FutureBound).UnixMilli(),
					0,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			expectedErr: chain.ErrTimestampTooLate,
		},
		{
			name:           "failed to get parent height",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrFailedToFetchParentHeight,
		},
		{
			name:           "failed to parse parent height",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    {},
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrFailedToParseParentHeight,
		},
		{
			name:           "block height is not one more than parent height",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, parentRoot ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					2,
					nil,
					parentRoot,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrInvalidBlockHeight,
		},
		{
			name:           "failed to get timestamp",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:    {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrFailedToFetchParentTimestamp,
		},
		{
			name:           "failed to parse timestamp",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: {},
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrFailedToParseParentTimestamp,
		},
		{
			name:           "non-empty block - timestamp less than parent timestamp with gap",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, parentRoot ids.ID) *chain.StatelessBlock {
				tx, err := chain.NewTransaction(
					chain.Base{
						Timestamp: utils.UnixRMilli(
							testRules.GetMinEmptyBlockGap(),
							testRules.GetValidityWindow(),
						),
					},
					[]chain.Action{},
					chaintest.NewDummyTestAuth(),
				)
				r.NoError(err)

				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinBlockGap()-1,
					1,
					[]*chain.Transaction{tx},
					parentRoot,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrTimestampTooEarly,
		},
		{
			name:           "empty block - timestamp less than parent timestamp with gap",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, parentRoot ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap()-1,
					1,
					nil,
					parentRoot,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrTimestampTooEarlyEmptyBlock,
		},
		{
			name:           "failed to get fee",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrFailedToFetchParentFee,
		},
		{
			name: "fails replay protection",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{
				OnVerifyExpiryReplayProtection: func(context.Context, validitywindow.ExecutionBlock[*chain.Transaction]) error {
					return errMockVerifyExpiryReplayProtection
				},
			},
			isNormalOp: true,
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, root ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					root,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: errMockVerifyExpiryReplayProtection,
		},
		{
			name:           "failed to execute txs",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, parentRoot ids.ID) *chain.StatelessBlock {
				tx, err := chain.NewTransaction(
					chain.Base{
						Timestamp: utils.UnixRMilli(
							testRules.GetMinEmptyBlockGap(),
							testRules.GetValidityWindow(),
						),
					},
					[]chain.Action{
						&chaintest.TestAction{
							NumComputeUnits: 1,
							Start:           -1,
							End:             -1,
							SpecifiedStateKeys: []string{
								"",
							},
							SpecifiedStateKeyPermissions: []state.Permissions{
								state.None,
							},
						},
					},
					chaintest.NewDummyTestAuth(),
				)
				r.NoError(err)

				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinBlockGap(),
					1,
					[]*chain.Transaction{tx},
					parentRoot,
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrInvalidKeyValue,
		},
		{
			name:           "state root mismatch",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, _ ids.ID) *chain.StatelessBlock {
				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinEmptyBlockGap(),
					1,
					nil,
					ids.GenerateTestID(),
					&block.Context{},
				)
				r.NoError(err)
				return block
			},
			expectedErr: chain.ErrStateRootMismatch,
		},
		{
			name:           "invalid transaction signature",
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			newViewF: func(r *require.Assertions) merkledb.View {
				v, err := createTestView(map[string][]byte{
					heightKey:    binary.BigEndian.AppendUint64(nil, 0),
					timestampKey: binary.BigEndian.AppendUint64(nil, 0),
					feeKey:       {},
				})
				r.NoError(err)
				return v
			},
			newBlockF: func(r *require.Assertions, parentRoot ids.ID) *chain.StatelessBlock {
				p, err := ed25519.GeneratePrivateKey()
				r.NoError(err)

				tx, err := chain.NewTransaction(
					chain.Base{
						Timestamp: utils.UnixRMilli(
							testRules.GetMinEmptyBlockGap(),
							testRules.GetValidityWindow(),
						),
					},
					[]chain.Action{},
					&auth.ED25519{
						Signer: p.PublicKey(),
					},
				)
				r.NoError(err)

				block, err := chain.NewStatelessBlock(
					ids.Empty,
					testRules.GetMinBlockGap(),
					1,
					[]*chain.Transaction{tx},
					parentRoot,
					&block.Context{},
				)
				r.NoError(err)
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
				&genesis.ImmutableRuleFactory{Rules: testRules},
				workers.NewSerial(),
				&mockAuthVM{},
				testMetadataManager,
				chaintest.NewTestBalanceHandler(nil, nil),
				tt.validityWindow,
				metrics,
				chain.NewDefaultConfig(),
			)

			view := tt.newViewF(r)
			root, err := view.GetMerkleRoot(ctx)
			r.NoError(err)

			block := tt.newBlockF(r, root)

			_, err = processor.Execute(
				ctx,
				view,
				chain.NewExecutionBlock(block),
				tt.isNormalOp,
			)
			r.ErrorIs(err, tt.expectedErr)
		})
	}
}

func createTestView(mp map[string][]byte) (merkledb.View, error) {
	db, err := merkledb.New(
		context.Background(),
		memdb.New(),
		merkledb.Config{
			BranchFactor: merkledb.BranchFactor16,
			Tracer:       trace.Noop,
		},
	)
	if err != nil {
		return nil, err
	}

	for key, value := range mp {
		if err := db.Put([]byte(key), value); err != nil {
			return nil, err
		}
	}

	return db, nil
}

type mockAuthVM struct{}

func (*mockAuthVM) GetAuthBatchVerifier(uint8, int, int) (chain.AuthBatchVerifier, bool) {
	return nil, false
}

func (*mockAuthVM) Logger() logging.Logger {
	panic("unimplemented")
}

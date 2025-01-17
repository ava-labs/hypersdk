// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/internal/validitywindow/validitywindowtest"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/metadata"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	feeKey = string(chain.FeeKey([]byte{2}))

	errMockAuth           = errors.New("mock auth error")
	errMockValidityWindow = errors.New("mock validity window error")
)

func isRepeatFuncGenerator(bits set.Bits, err error) func(
	ctx context.Context,
	parentBlk validitywindow.ExecutionBlock[*chain.Transaction],
	containers []*chain.Transaction,
	currentTime int64,
) (set.Bits, error) {
	return func(
		context.Context,
		validitywindow.ExecutionBlock[*chain.Transaction],
		[]*chain.Transaction,
		int64,
	) (set.Bits, error) {
		return bits, err
	}
}

func TestPreExecutor(t *testing.T) {
	testRules := genesis.NewDefaultRules()
	ruleFactory := genesis.ImmutableRuleFactory{
		Rules: testRules,
	}
	validTx := &chain.Transaction{
		TransactionData: chain.TransactionData{
			Base: &chain.Base{
				Timestamp: utils.UnixRMilli(
					time.Now().UnixMilli(),
					testRules.GetValidityWindow(),
				),
			},
		},
		Auth: &mockAuth{
			start: -1,
			end:   -1,
		},
	}

	tests := []struct {
		name           string
		state          map[string][]byte
		tx             *chain.Transaction
		validityWindow chain.ValidityWindow
		err            error
	}{
		{
			name: "valid test case",
			state: map[string][]byte{
				feeKey: {},
			},
			tx:             validTx,
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
		},
		{
			name: "raw fee doesn't exist",
			tx:   validTx,
			err:  chain.ErrFailedToFetchFee,
		},
		{
			name: "validity window error",
			tx:   validTx,
			state: map[string][]byte{
				feeKey: {},
			},
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{
				OnIsRepeat: isRepeatFuncGenerator(set.NewBits(), errMockValidityWindow),
			},
			err: errMockValidityWindow,
		},
		{
			name: "duplicate transaction",
			tx:   validTx,
			state: map[string][]byte{
				feeKey: {},
			},
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{
				OnIsRepeat: isRepeatFuncGenerator(set.NewBits(0), nil),
			},
			err: chain.ErrDuplicateTx,
		},
		{
			name: "tx state keys are invalid",
			state: map[string][]byte{
				feeKey: {},
			},
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Actions: []chain.Action{
						&mockAction{
							stateKeys: state.Keys{
								"": state.None,
							},
						},
					},
				},
				Auth: &mockAuth{},
			},
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			err:            chain.ErrInvalidKeyValue,
		},
		{
			name: "verify auth error",
			state: map[string][]byte{
				feeKey: {},
			},
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{},
				},
				Auth: &mockAuth{
					verifyError: errMockAuth,
				},
			},
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			err:            errMockAuth,
		},
		{
			name: "transaction pre-execute error",
			state: map[string][]byte{
				feeKey: {},
			},
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{},
				},
				Auth: &mockAuth{},
			},
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
			err:            chain.ErrTimestampTooLate,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()

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

			preExecutor := chain.NewPreExecutor(
				&ruleFactory,
				tt.validityWindow,
				metadata.NewDefaultManager(),
				&mockBalanceHandler{},
			)

			r.ErrorIs(
				preExecutor.PreExecute(
					ctx,
					nil,
					db,
					tt.tx,
				), tt.err,
			)
		})
	}
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/metadata"
	"github.com/ava-labs/hypersdk/utils"
)

var (
	feeKey = string(chain.FeeKey([]byte{2}))

	errMockAuth           = errors.New("mock auth error")
	errMockValidityWindow = errors.New("mock validity window error")

	_ chain.Auth           = (*mockAuth)(nil)
	_ chain.BalanceHandler = (*mockBalanceHandler)(nil)
	_ chain.ValidityWindow = (*mockValidityWindow)(nil)
)

type mockValidityWindow struct {
	isRepeatError error
	setBits       []int
}

func (*mockValidityWindow) Accept(validitywindow.ExecutionBlock[*chain.Transaction]) {
	panic("unimplemented")
}

func (m *mockValidityWindow) IsRepeat(context.Context, validitywindow.ExecutionBlock[*chain.Transaction], []*chain.Transaction, int64) (set.Bits, error) {
	return set.NewBits(m.setBits...), m.isRepeatError
}

func (*mockValidityWindow) VerifyExpiryReplayProtection(context.Context, validitywindow.ExecutionBlock[*chain.Transaction], int64) error {
	panic("unimplemented")
}

func TestPreExecutor(t *testing.T) {
	testRules := genesis.NewDefaultRules()
	ruleFactory := genesis.ImmutableRuleFactory{
		Rules: testRules,
	}

	tests := []struct {
		name           string
		state          map[string][]byte
		tx             *chain.Transaction
		validityWindow chain.ValidityWindow
		verifyAuth     bool
		err            error
	}{
		{
			name: "valid test case",
			state: map[string][]byte{
				feeKey: {},
			},
			tx: &chain.Transaction{
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
			},
			validityWindow: &mockValidityWindow{},
		},
		{
			name: "raw fee doesn't exist",
			err:  database.ErrNotFound,
		},
		{
			name: "validity window error",
			tx:   &chain.Transaction{},
			state: map[string][]byte{
				feeKey: {},
			},
			validityWindow: &mockValidityWindow{
				isRepeatError: errMockValidityWindow,
			},
			err: errMockValidityWindow,
		},
		{
			name: "duplicate transaction",
			tx:   &chain.Transaction{},
			state: map[string][]byte{
				feeKey: {},
			},
			validityWindow: &mockValidityWindow{
				setBits: []int{0},
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
			validityWindow: &mockValidityWindow{},
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
			validityWindow: &mockValidityWindow{},
			verifyAuth:     true,
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
			validityWindow: &mockValidityWindow{},
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
					tt.verifyAuth,
				), tt.err,
			)
		})
	}
}

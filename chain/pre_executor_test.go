// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/utils/set"
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
			name: "valid tx",
			state: map[string][]byte{
				feeKey: {},
			},
			tx:             validTx,
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{},
		},
		{
			name: "raw fee missing",
			tx:   validTx,
			err:  chain.ErrFailedToFetchParentFee,
		},
		{
			name: "validity window error",
			tx:   validTx,
			state: map[string][]byte{
				feeKey: {},
			},
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{
				OnIsRepeat: func(context.Context, validitywindow.ExecutionBlock[*chain.Transaction], []*chain.Transaction, int64) (set.Bits, error) {
					return set.NewBits(), errMockValidityWindow
				},
			},
			err: errMockValidityWindow,
		},
		{
			name: "duplicate tx",
			tx:   validTx,
			state: map[string][]byte{
				feeKey: {},
			},
			validityWindow: &validitywindowtest.MockTimeValidityWindow[*chain.Transaction]{
				OnIsRepeat: func(context.Context, validitywindow.ExecutionBlock[*chain.Transaction], []*chain.Transaction, int64) (set.Bits, error) {
					return set.NewBits(0), nil
				},
			},
			err: chain.ErrDuplicateTx,
		},
		{
			name: "invalid state keys",
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
			name: "tx pre-execute error",
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
			err:            validitywindow.ErrTimestampExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()

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
					state.ImmutableStorage(tt.state),
					tt.tx,
				), tt.err,
			)
		})
	}
}

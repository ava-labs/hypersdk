// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/state"
)

var (
	_ chain.MetadataManager                         = (*mockMetadataManager)(nil)
	_ chain.Action                                  = (*mockAction1)(nil)
	_ chain.Auth                                    = (*mockAuth)(nil)
	_ chain.BalanceHandler                          = (*mockBalanceHandler)(nil)
	_ state.View                                    = (*abstractMockView)(nil)
	_ validitywindow.ChainIndex[*chain.Transaction] = (*mockChainIndex)(nil)
)

var (
	errMockView           = errors.New("mock view error")
	errMockExecutionBlock = errors.New("mock execution block error")
	errMockAuth           = errors.New("mock auth error")
)

type abstractMockView struct{}

func (*abstractMockView) GetValue(context.Context, []byte) ([]byte, error) {
	panic("unimplemented")
}

func (*abstractMockView) GetMerkleRoot(context.Context) (ids.ID, error) {
	panic("unimplemented")
}

func (*abstractMockView) NewView(context.Context, merkledb.ViewChanges) (merkledb.View, error) {
	panic("unimplemented")
}

type mockView1 struct {
	abstractMockView
}

func (*mockView1) GetValue(context.Context, []byte) ([]byte, error) {
	return nil, errMockView
}

type mockView2 struct {
	abstractMockView
}

func (*mockView2) GetValue(context.Context, []byte) ([]byte, error) {
	return []byte{}, nil
}

type mockMetadataManager struct{}

func (*mockMetadataManager) FeePrefix() []byte {
	return []byte{}
}

func (*mockMetadataManager) HeightPrefix() []byte {
	return []byte{}
}

func (*mockMetadataManager) TimestampPrefix() []byte {
	return []byte{}
}

type mockChainIndex struct {
	err error
}

func (m *mockChainIndex) GetExecutionBlock(context.Context, ids.ID) (validitywindow.ExecutionBlock[*chain.Transaction], error) {
	return nil, m.err
}

type mockAction1 struct {
	abstractMockAction
	stateKeys state.Keys
}

func (*mockAction1) GetTypeID() uint8 {
	return 1
}

func (m *mockAction1) StateKeys(codec.Address, ids.ID) state.Keys {
	return m.stateKeys
}

type mockAuth struct{}

func (*mockAuth) Actor() codec.Address {
	return codec.Address{}
}

func (*mockAuth) ComputeUnits(chain.Rules) uint64 {
	panic("unimplemented")
}

func (*mockAuth) GetTypeID() uint8 {
	panic("unimplemented")
}

func (*mockAuth) Marshal(*codec.Packer) {
	panic("unimplemented")
}

func (*mockAuth) Size() int {
	panic("unimplemented")
}

func (*mockAuth) Sponsor() codec.Address {
	return codec.Address{}
}

func (*mockAuth) ValidRange(chain.Rules) (int64, int64) {
	panic("unimplemented")
}

func (*mockAuth) Verify(context.Context, []byte) error {
	return errMockAuth
}

type mockBalanceHandler struct{}

func (*mockBalanceHandler) AddBalance(context.Context, codec.Address, state.Mutable, uint64) error {
	panic("unimplemented")
}

func (*mockBalanceHandler) CanDeduct(context.Context, codec.Address, state.Immutable, uint64) error {
	panic("unimplemented")
}

func (*mockBalanceHandler) Deduct(context.Context, codec.Address, state.Mutable, uint64) error {
	panic("unimplemented")
}

func (*mockBalanceHandler) GetBalance(context.Context, codec.Address, state.Immutable) (uint64, error) {
	panic("unimplemented")
}

func (*mockBalanceHandler) SponsorStateKeys(codec.Address) state.Keys {
	return state.Keys{}
}

func TestPreExecutor(t *testing.T) {
	ruleFactory := genesis.ImmutableRuleFactory{Rules: genesis.NewDefaultRules()}

	tests := []struct {
		name string

		view       state.View
		tx         *chain.Transaction
		chainIndex validitywindow.ChainIndex[*chain.Transaction]
		height     uint64
		verifyAuth bool
		err        error
	}{
		{
			name: "raw fee doesn't exist",
			view: &mockView1{},
			err:  errMockView,
		},
		{
			name: "repeat error",
			view: &mockView2{},
			tx:   &chain.Transaction{},
			chainIndex: &mockChainIndex{
				err: errMockExecutionBlock,
			},
			height: 1,
			err:    errMockExecutionBlock,
		},
		{
			name: "tx state keys are invalid",
			view: &mockView2{},
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Actions: []chain.Action{
						&mockAction1{
							stateKeys: state.Keys{
								"": state.None,
							},
						},
					},
				},
				Auth: &mockAuth{},
			},
			chainIndex: &mockChainIndex{},
			err:        chain.ErrInvalidKeyValue,
		},
		{
			name: "verify auth error",
			view: &mockView2{},
			tx: &chain.Transaction{
				TransactionData: chain.TransactionData{
					Base: &chain.Base{},
					Actions: []chain.Action{
						&mockAction1{},
					},
				},
				Auth: &mockAuth{},
			},
			chainIndex: &mockChainIndex{},
			verifyAuth: true,
			err:        errMockAuth,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := require.New(t)
			ctx := context.Background()

			parentBlock, err := chain.NewExecutionBlock(
				&chain.StatelessBlock{
					Hght:   tt.height,
					Tmstmp: time.Now().UnixMilli(),
				},
			)
			r.NoError(err)

			preExecutor := chain.NewPreExecutor(
				&ruleFactory,
				validitywindow.NewTimeValidityWindow(nil, trace.Noop, tt.chainIndex),
				&mockMetadataManager{},
				&mockBalanceHandler{},
			)

			r.ErrorIs(
				preExecutor.PreExecute(
					ctx,
					parentBlock,
					tt.view,
					tt.tx,
					tt.verifyAuth,
				), tt.err,
			)
		})
	}
}

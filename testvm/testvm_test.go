// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package testvm

import (
	"context"
	"testing"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/trace"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type StateManager struct{}

func (StateManager) SponsorStateKeys(addr codec.Address) state.Keys {
	return state.Keys{}
}

func (StateManager) CanDeduct(ctx context.Context, addr codec.Address, im state.Immutable, amount uint64) error {
	return nil
}

func (StateManager) Deduct(ctx context.Context, addr codec.Address, mu state.Mutable, amount uint64) error {
	return nil
}

func (StateManager) HeightKey() []byte {
	return nil
}

func (StateManager) TimestampKey() []byte {
	return nil
}

func (StateManager) FeeKey() []byte {
	return nil
}

func TestEnvSnapshot(t *testing.T) {
	require := require.New(t)

	config := TestConfig{
		StateManager: StateManager{},
		TracerConfig: trace.Config{Enabled: false},
	}
	ctx := context.Background()
	vm, err := NewTestVM(ctx, config)
	require.NoError(err)
	require.Equal(uint64(1), vm.Height())
	snapId, err := vm.SnapshotSave()
	require.NoError(err)
	vm.SetHeight(10)
	require.Equal(uint64(10), vm.Height())
	err = vm.SnapshotRevert(snapId)
	require.NoError(err)
	require.Equal(uint64(1), vm.Height())
}

func TestBlockBuildingTrigger(t *testing.T) {
	require := require.New(t)

	config := TestConfig{
		StateManager:    StateManager{},
		TracerConfig:    trace.Config{Enabled: false},
		BlockProduction: BlockProduction{Type: Trigger},
	}
	ctx := context.Background()
	vm, err := NewTestVM(ctx, config)
	require.NoError(err)
	require.Equal(uint64(1), vm.Height())
	err = vm.BuildBlock()
	require.NoError(err)
	require.Equal(uint64(2), vm.Height())
}

func TestBlockBuildingBatch(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rules := chain.NewMockRules(ctrl)
	rules.EXPECT().GetBaseComputeUnits().Return(uint64(60)).AnyTimes()
	rules.EXPECT().GetWindowTargetUnits().Return([5]uint64{}).AnyTimes()
	rules.EXPECT().GetUnitPriceChangeDenominator().Return([5]uint64{}).AnyTimes()
	rules.EXPECT().GetMinUnitPrice().Return([5]uint64{}).AnyTimes()

	config := TestConfig{
		StateManager: StateManager{},
		TracerConfig: trace.Config{Enabled: false},
		MaxUnits:     [5]uint64{255, 255, 255, 255, 255},
		BlockProduction: BlockProduction{
			Type:  MinTransactionBatch,
			Value: 2,
		},
		Rules: rules,
	}
	ctx := context.Background()
	vm, err := NewTestVM(ctx, config)
	require.NoError(err)
	require.Equal(uint64(1), vm.Height())
	err = vm.BuildBlock()
	require.ErrorIs(NotEnoughTransactions, err)
	key, err := secp256r1.GeneratePrivateKey()
	require.NoError(err)
	factory := auth.NewSECP256R1Factory(key)
	secpAuth, err := factory.Sign([]byte{})
	require.NoError(err)
	tx := chain.Transaction{
		Base:    &chain.Base{},
		Actions: []chain.Action{},
		Auth:    secpAuth,
	}
	_, err = vm.RunTransaction(ctx, tx)
	require.NoError(err)
	err = vm.BuildBlock()
	require.ErrorIs(NotEnoughTransactions, err)
	_, err = vm.RunTransaction(ctx, tx)
	require.NoError(err)
	err = vm.BuildBlock()
	require.NoError(err)
}

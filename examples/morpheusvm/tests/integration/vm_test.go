package integration_test

import (
	"context"
	"encoding/binary"
	"testing"
	"time"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/testvm"
	"github.com/ava-labs/hypersdk/trace"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestVmIntegration(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	rules := chain.NewMockRules(ctrl)
	rules.EXPECT().GetBaseComputeUnits().Return(uint64(60)).AnyTimes()
	rules.EXPECT().GetStorageKeyReadUnits().Return(uint64(60)).AnyTimes()
	rules.EXPECT().GetStorageKeyWriteUnits().Return(uint64(60)).AnyTimes()
	rules.EXPECT().GetStorageKeyAllocateUnits().Return(uint64(60)).AnyTimes()
	rules.EXPECT().GetUnitPriceChangeDenominator().Return([5]uint64{}).AnyTimes()
	rules.EXPECT().GetStorageValueAllocateUnits().Return(uint64(60)).AnyTimes()
	rules.EXPECT().GetStorageValueWriteUnits().Return(uint64(60)).AnyTimes()
	rules.EXPECT().GetStorageValueReadUnits().Return(uint64(60)).AnyTimes()
	rules.EXPECT().GetWindowTargetUnits().Return([5]uint64{}).AnyTimes()
	rules.EXPECT().GetMaxOutputsPerAction().Return(uint8(60)).AnyTimes()
	rules.EXPECT().GetMinUnitPrice().Return([5]uint64{}).AnyTimes()

	config := testvm.TestConfig{
		BlockProduction: testvm.BlockProduction{Type: testvm.Trigger},
		MaxUnits:        [5]uint64{255, 255, 255, 255, 255},
		StateManager:    &storage.StateManager{},
		TracerConfig:    trace.Config{Enabled: false},
		Rules:           rules,
	}
	ctx := context.Background()
	vm, err := testvm.Init(ctx, config)
	require.NoError(err)

	key, err := secp256r1.GeneratePrivateKey()
	require.NoError(err)
	factory := auth.NewSECP256R1Factory(key)
	secpAuth, err := factory.Sign([]byte{})
	require.NoError(err)

	from := auth.NewSECP256R1Address(key.PublicKey())
	require.NoError(err)
	fromKey := storage.BalanceKey(from)
	to := codec.EmptyAddress
	toKey := storage.BalanceKey(to)
	vm.Insert(fromKey, binary.BigEndian.AppendUint64(nil, 1))
	vm.Insert(toKey, binary.BigEndian.AppendUint64(nil, 0))

	tx := chain.Transaction{
		Base: &chain.Base{
			Timestamp: time.Now().Unix(),
			ChainID:   [32]byte{},
			MaxFee:    100,
		},
		Actions: []chain.Action{&actions.Transfer{
			To:    to,
			Value: 1,
		}},
		Auth: secpAuth,
	}

	result, err := vm.RunTransaction(ctx, tx)
	require.NoError(err)
	require.NotNil(result)
	require.Equal("", string(result.Error))
	require.True(result.Success)
	require.Equal(0, len(result.Error))
	require.NotEqual(0, len(result.Outputs))

	fromBalRaw, err := vm.Get(fromKey)
	require.NoError(err)
	var fromBal uint64
	binary.BigEndian.PutUint64(fromBalRaw, fromBal)
	require.Equal(uint64(0), fromBal)

	toBalRaw, err := vm.Get(toKey)
	require.NoError(err)
	var toBal uint64
	binary.BigEndian.PutUint64(toBalRaw, toBal)
	require.Equal(uint64(0), toBal)
}

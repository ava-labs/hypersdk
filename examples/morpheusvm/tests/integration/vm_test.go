package integration_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
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
	rules.EXPECT().GetBaseComputeUnits().Return(uint64(60))
	rules.EXPECT().GetStorageKeyReadUnits().Return(uint64(60)).AnyTimes()
	rules.EXPECT().GetStorageKeyWriteUnits().Return(uint64(60)).AnyTimes()
	rules.EXPECT().GetStorageKeyAllocateUnits().Return(uint64(60)).AnyTimes()
	rules.EXPECT().GetStorageValueReadUnits().Return(uint64(60)).AnyTimes()
	rules.EXPECT().GetStorageValueAllocateUnits().Return(uint64(60)).AnyTimes()
	rules.EXPECT().GetStorageValueWriteUnits().Return(uint64(60)).AnyTimes()

	vm := testvm.TestVM{}
	config := testvm.TestConfig{
		BlockProduction: testvm.BlockProduction{Type: testvm.Trigger},
		MaxUnits:        [5]uint64{},
		StateManager:    &storage.StateManager{},
		TracerConfig:    trace.Config{Enabled: false},
		Rules:           rules,
	}
	ctx := context.Background()
	vm.Init(ctx, config)
	key, err := secp256r1.GeneratePrivateKey()
	require.NoError(err)
	factory := auth.NewSECP256R1Factory(key)
	auth, err := factory.Sign([]byte{})
	require.NoError(err)
	tx := chain.Transaction{
		Base: &chain.Base{
			Timestamp: time.Now().Unix(),
			ChainID:   [32]byte{},
			MaxFee:    100,
		},
		Actions: []chain.Action{&actions.Transfer{}},
		Auth:    auth,
	}
	result, err := vm.RunTransaction(ctx, tx)
	require.NoError(err)
	fmt.Println(result)
}

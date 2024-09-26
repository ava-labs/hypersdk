package actions

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
	"github.com/stretchr/testify/require"
)

func TestCallAction(t *testing.T) {
	addr := codectest.NewRandomAddress()
	counterBytes, err := LoadBytes("counter.wasm")
	require.NoError(t, err)

	// value to get
	addressOf := codec.CreateAddress(consts.AddressID, ids.GenerateTestID())

	// this will be the same when the contract is deployed
	counterID := sha256.Sum256(counterBytes)
	expectedAddress := storage.GetAccountAddress(counterID[:], []byte{0})

	config := runtime.NewConfig()
	config.SetDebugInfo(true)

	rt := runtime.NewRuntime(config, logging.NewLogger("test"))
	getValueArgs, err := SerializeArgs(addressOf)
	if err != nil {
		t.Fatal(err)
	}

	tests := []chaintest.ActionTest{
		{
			Name:  "CallCounter",
			Actor: addr,
			Action: &Call{
				ContractAddress: expectedAddress,
				Value: 0,
				FunctionName: "get_value",
				Args: getValueArgs,
				Fuel: 1000000000,
				r: rt,
			},
			State: func() state.Mutable {
				// deploy the counterBytes contract
				store := chaintest.NewInMemoryStore()
				deploy := &Deploy{
					ContractBytes: counterBytes,
					CreationData:  []byte{0},
				}
				deployOutput, err := deploy.Execute(context.Background(), nil, store, 0, addr, ids.Empty)
				deployedContractID := deployOutput.(*DeployOutput).ID
				deployedAddress := deployOutput.(*DeployOutput).Account

				require.NoError(t, err)
				require.Equal(t, runtime.ContractID(counterID[:]), deployedContractID)
				require.Equal(t, expectedAddress, deployedAddress)

				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				// get the stored value
			},
			ExpectedOutputs: &CallOutput{
				result: []byte{0, 0, 0, 0, 0, 0, 0, 0},
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}
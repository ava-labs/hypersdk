package actions

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/examples/updated-vm-with-contracts/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/contracts/runtime"
	"github.com/stretchr/testify/require"
)

func TestCallAction(t *testing.T) {
	addr := codectest.NewRandomAddress()
	counterBytes, err := LoadBytes("counter.wasm")
	require.NoError(t, err)

	// this will be the same when the contract is deployed
	counterID := sha256.Sum256(counterBytes)
	expectedAddress := storage.GetAccountAddress(counterID[:], []byte{0})

	config := runtime.NewConfig()
	config.SetDebugInfo(true)
	rt := runtime.NewRuntime(config, logging.NewLogger("test"))
	fuel := uint64(1000000000)
	
	getValueArgs, err := SerializeArgs(addr)
	require.NoError(t, err)

	incArgs, err := SerializeArgs(addr, uint64(1))
	require.NoError(t, err)

	trueResult, err := SerializeArgs(true)
	require.NoError(t, err)
	
	tests := []chaintest.ActionTest{
		{
			Name:  "CallCounterGetValue",
			Actor: addr,
			Action: &Call{
				ContractAddress: expectedAddress,
				Value:           0,
				FunctionName:    "get_value",
				Args:            getValueArgs,
				Fuel:            fuel,
				r:               rt,
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
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {},
			ExpectedOutputs: &CallOutput{
				result: []byte{0, 0, 0, 0, 0, 0, 0, 0},
			},
		},
		{
			Name:  "CallCounterInc",
			Actor: addr,
			Action: &Call{
				ContractAddress: expectedAddress,
				Value:           0,
				FunctionName:    "inc",
				Args:            incArgs,
				Fuel:            fuel,
				r:               rt,
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
				call := &Call{
					ContractAddress: expectedAddress,
					Value:           0,
					FunctionName:    "get_value",
					Args:            getValueArgs,
					Fuel:            fuel,
					r:               rt,
				}
				output, err := call.Execute(ctx, nil, store, 0, addr, ids.Empty)
				require.NoError(t, err)
				expectedValue, err := SerializeArgs(uint64(1))
				require.NoError(t, err)
				require.Equal(t, expectedValue, output.(*CallOutput).result)
			},
			ExpectedOutputs: &CallOutput{
				result: trueResult,
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

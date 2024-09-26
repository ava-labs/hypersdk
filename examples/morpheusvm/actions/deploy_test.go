package actions

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
	"github.com/stretchr/testify/require"
)

func TestDeployAction(t *testing.T) {
	addr := codectest.NewRandomAddress()
	counterBytes, err := LoadBytes("counter.wasm")
	require.NoError(t, err)
	contractID := sha256.Sum256(counterBytes)

	tests := []chaintest.ActionTest{
		{
			Name:  "Deploy",
			Actor: addr,
			Action: &Deploy{
				ContractBytes: counterBytes,
				CreationData:  []byte{0},
			},
			State: func() state.Mutable {
				store := chaintest.NewInMemoryStore()
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				account := storage.GetAccountAddress(contractID[:], []byte{0})
				contractAccountKey := storage.AccountContractIDKey(account)
				contractBytesKey := storage.ContractBytesKey(contractID[:])

				_, err := store.GetValue(ctx, contractBytesKey)
				require.NoError(t, err)

				_, err = store.GetValue(ctx, contractAccountKey)
				require.NoError(t, err)
			},
			ExpectedOutputs: &DeployOutput{
				ID: contractID[:],
				Account: storage.GetAccountAddress(contractID[:], []byte{0}),
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}
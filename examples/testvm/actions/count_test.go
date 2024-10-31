// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/examples/testvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

func TestCountAction(t *testing.T) {
	addr := codectest.NewRandomAddress()
	anotherAddr := codectest.NewRandomAddress()

	store := chaintest.NewInMemoryStore()
	tests := []chaintest.ActionTest{
		{
			Name:  "IncrementZero",
			Actor: addr,
			Action: &Count{
				Amount: 0,
				Address: addr,
			},
			State: store,
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				count, err := storage.GetCounter(ctx, store, addr)
				require.NoError(t, err)
				require.Equal(t, uint64(0), count)
			},
			ExpectedOutputs: &CountResult{
				Count: 0,
			},
		},
		{
			Name:  "IncrementOne",
			Actor: addr,
			Action: &Count{
				Amount: 1,
				Address:    addr,
			},
			State: store,
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				count, err := storage.GetCounter(ctx, store, addr)
				require.NoError(t, err)
				require.Equal(t, uint64(1), count)
			},
			ExpectedOutputs: &CountResult{
				Count: 1,
			},
		},
		{
			Name:  "IncrementExistingState",
			Actor: addr,
			Action: &Count{
				Amount: 100,
				Address:    addr,
			},
			State: store,
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				count, err := storage.GetCounter(ctx, store, addr)
				require.NoError(t, err)
				require.Equal(t, uint64(101), count)
			},
			ExpectedOutputs: &CountResult{
				Count: 101,
			},
		},
		{
			Name:  "IncrementDifferentActor",
			Actor: addr,
			Action: &Count{
				Amount: 1,
				Address:    anotherAddr,
			},
			State: store,
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				count, err := storage.GetCounter(ctx, store, addr)
				require.NoError(t, err)
				require.Equal(t, uint64(101), count)

				count, err = storage.GetCounter(ctx, store, anotherAddr)
				require.NoError(t, err)
				require.Equal(t, uint64(1), count)
			},
			ExpectedOutputs: &CountResult{
				Count: 1,
			},
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

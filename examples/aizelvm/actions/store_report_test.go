// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/stretchr/testify/require"
)

func TestStoreReportAction(t *testing.T) {
	actor := codec.CreateAddress(0, ids.GenerateTestID())

	tests := []chaintest.ActionTest{
		{
			Name:  "ValidStoreReport",
			Actor: actor,
			Action: &StoreReport{
				ReqId:  123,
				Report: "Sample",
			},
			State: chaintest.NewInMemoryStore(),
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				value, err := store.GetValue(ctx, StorageKey("123"))
				require.NoError(t, err)
				require.Equal(t, "Sample", string(value))
			},
			ExpectedOutputs: func() []byte {
				result := &StoreReportOutput{
					Stored: true,
				}
				return result.Bytes()
			}(),
		},
		{
			Name:  "EmptyReport",
			Actor: actor,
			Action: &StoreReport{
				ReqId:  123,
				Report: "",
			},
			State:       chaintest.NewInMemoryStore(),
			ExpectedErr: ErrInvalidReport,
		},
		{
			Name:  "ReportTooLong",
			Actor: actor,
			Action: &StoreReport{
				ReqId:  123,
				Report: string(make([]byte, 1025)), // 1KB + 1
			},
			State:       chaintest.NewInMemoryStore(),
			ExpectedErr: ErrInvalidReport,
		},
		{
			Name:  "OverwriteReport",
			Actor: actor,
			Action: &StoreReport{
				ReqId:  123,
				Report: "Updated",
			},
			State: func() state.Mutable {
				store := chaintest.NewInMemoryStore()
				require.NoError(t, store.Insert(context.Background(), StorageKey("123"), []byte("Original")))
				return store
			}(),
			Assertion: func(ctx context.Context, t *testing.T, store state.Mutable) {
				value, err := store.GetValue(ctx, StorageKey("123"))
				require.NoError(t, err)
				require.Equal(t, "Updated", string(value))
			},
			ExpectedOutputs: func() []byte {
				result := &StoreReportOutput{
					Stored: true,
				}
				return result.Bytes()
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			tt.Run(context.Background(), t)
		})
	}
}

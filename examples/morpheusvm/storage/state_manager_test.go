// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec/codectest"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/storage"
	"github.com/ava-labs/hypersdk/state"
)

func TestBalanceHandlerStateKeys(t *testing.T) {
	addr := codectest.NewRandomAddress()

	test := chaintest.BalanceHandlerStateKeyTest{
		Name:           "SimpleStateKeys",
		BalanceHandler: &storage.BalanceHandler{},
		Actor:          addr,
		Amount:         1,
	}

	test.Run(context.Background(), t)
}

func TestBalanceHandlerFunds(t *testing.T) {
	addr := codectest.NewRandomAddress()
	r := require.New(t)

	tests := []chaintest.BalanceHandlerFundsTest{
		{
			Name:           "Insufficient funds",
			BalanceHandler: &storage.BalanceHandler{},
			Actor:          addr,
			State:          chaintest.NewInMemoryStore(),
			Amount:         1,
			CreateAccount:  false,
			Method:         chaintest.TestDeduct,
			ExpectedError:  storage.ErrInvalidBalance,
		},
		{
			Name:            "SimpleAddBalance",
			BalanceHandler:  &storage.BalanceHandler{},
			Actor:           addr,
			State:           chaintest.NewInMemoryStore(),
			Amount:          1,
			CreateAccount:   true,
			Method:          chaintest.TestAddBalance,
			ExpectedBalance: uint64(1),
			ExpectedError:   nil,
		},
		{
			Name:           "SimpleCanDeduct",
			BalanceHandler: &storage.BalanceHandler{},
			Actor:          addr,
			State: func() state.Mutable {
				s := chaintest.NewInMemoryStore()
				_, err := storage.AddBalance(
					context.Background(),
					s,
					addr,
					1,
					true,
				)
				r.NoError(err)
				return s
			}(),
			Amount:          1,
			ExpectedBalance: 1,
		},
		{
			Name:           "SimpleDeduct",
			BalanceHandler: &storage.BalanceHandler{},
			Actor:          addr,
			State: func() state.Mutable {
				s := chaintest.NewInMemoryStore()
				_, err := storage.AddBalance(
					context.Background(),
					s,
					addr,
					1,
					true,
				)
				r.NoError(err)
				return s
			}(),
			Amount:          uint64(1),
			ExpectedBalance: uint64(1),
		},
	}

	for _, tt := range tests {
		tt.Run(context.Background(), t)
	}
}

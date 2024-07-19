// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package unit_test

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

var _ state.Mutable = (*InMemoryStore)(nil)

// InMemoryStore is a storage that acts as a wrapper around a map and implements state.Mutable.
type InMemoryStore struct {
	Storage map[string][]byte
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		Storage: make(map[string][]byte),
	}
}

func (i *InMemoryStore) GetValue(_ context.Context, key []byte) ([]byte, error) {
	val, ok := i.Storage[string(key)]
	if !ok {
		return nil, database.ErrNotFound
	}
	return val, nil
}

func (i *InMemoryStore) Insert(_ context.Context, key []byte, value []byte) error {
	i.Storage[string(key)] = value
	return nil
}

func (i *InMemoryStore) Remove(_ context.Context, key []byte) error {
	delete(i.Storage, string(key))
	return nil
}

// ActionTest is a single parameterized test. It calls Execute on the action with the passed parameters
// and checks that all assertions pass.
type ActionTest struct {
	Name string

	Action chain.Action
	SetupActions []chain.Action

	Rules     chain.Rules
	State     state.Mutable
	Timestamp int64
	Actor     codec.Address
	ActionID  ids.ID

	ExpectedOutputs [][]byte
	ExpectedErr     error

	Assertion func(state.Mutable) bool
}

// Run execute all tests from the test suite and make sure all assertions pass.
func Run(t *testing.T, tests []ActionTest) {
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			require := require.New(t)

			if test.SetupActions != nil {
				for _, setupAction := range(test.SetupActions) {
					_, err := setupAction.Execute(context.TODO(), test.Rules, test.State, test.Timestamp, test.Actor, test.ActionID)
					require.NoError(err)
				}
			}

			output, err := test.Action.Execute(context.TODO(), test.Rules, test.State, test.Timestamp, test.Actor, test.ActionID)

			require.ErrorIs(err, test.ExpectedErr)
			require.Equal(output, test.ExpectedOutputs)

			if test.Assertion != nil {
				require.True(test.Assertion(test.State))
			}
		})
	}
}
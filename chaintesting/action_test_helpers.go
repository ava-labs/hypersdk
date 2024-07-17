// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintesting

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

var _ state.Mutable = (*inMemoryStore)(nil)

// inMemoryStore is a storage that acts as a wrapper around a map and implements state.Mutable.
type inMemoryStore struct {
	Storage map[string][]byte
}

func NewInMemoryStore() *inMemoryStore {
	return &inMemoryStore{
		Storage: make(map[string][]byte),
	}
}

func (s *inMemoryStore) GetValue(_ context.Context, key []byte) ([]byte, error) {
	val, ok := s.Storage[string(key)]
	if !ok {
		return nil, database.ErrNotFound
	}
	return val, nil
}

func (s *inMemoryStore) Insert(_ context.Context, key []byte, value []byte) error {
	s.Storage[string(key)] = value
	return nil
}

func (s *inMemoryStore) Remove(_ context.Context, key []byte) error {
	delete(s.Storage, string(key))
	return nil
}

// ActionTest is a single parameterized test. It calls Execute on the action with the passed parameters
// and checks that the assertions passes.
type ActionTest struct {
	Action chain.Action

	Rules     chain.Rules
	State     state.Mutable
	Timestamp int64
	Actor     codec.Address
	ActionID  ids.ID

	ExpectedOutputs [][]byte
	ExpectedErr     error

	Assertion func(state.Mutable) bool
}

type ActionTestSuite struct {
	Tests map[string]ActionTest
}

// Run execute all tests from the test suite and make sure that assertions passes.
func (suite *ActionTestSuite) Run(t *testing.T) {
	for name, test := range suite.Tests {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)

			output, err := test.Action.Execute(context.TODO(), test.Rules, test.State, test.Timestamp, test.Actor, test.ActionID)

			require.ErrorIs(err, test.ExpectedErr)
			require.Equal(output, test.ExpectedOutputs)

			if test.Assertion != nil {
				require.True(test.Assertion(test.State))
			}
		})
	}
}

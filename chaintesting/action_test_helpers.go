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

func (s *InMemoryStore) GetValue(_ context.Context, key []byte) ([]byte, error) {
	val, ok := s.Storage[string(key)]
	if !ok {
		return nil, database.ErrNotFound
	}
	return val, nil
}

func (s *InMemoryStore) Insert(_ context.Context, key []byte, value []byte) error {
	s.Storage[string(key)] = value
	return nil
}

func (s *InMemoryStore) Remove(_ context.Context, key []byte) error {
	delete(s.Storage, string(key))
	return nil
}

// ActionTest is a single parameterized test. It calls Execute on the action with the passed parameters
// and checks that all assertions pass.
type ActionTest struct {
	Action chain.Action

	Rules     chain.Rules
	State     state.Mutable
	Timestamp int64
	Actor     codec.Address
	ActionID  ids.ID

	ExpectedOutputs [][]byte
	ExpectedErr     error
}

type ActionTestSuite struct {
	Tests map[string]ActionTest
}

// Run execute all tests from the test suite and make sure all assertions pass.
func (suite *ActionTestSuite) Run(t *testing.T) {
	for name, test := range suite.Tests {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)

			output, err := test.Action.Execute(context.TODO(), test.Rules, test.State, test.Timestamp, test.Actor, test.ActionID)

			require.ErrorIs(err, test.ExpectedErr)
			require.Equal(output, test.ExpectedOutputs)
		})
	}
}

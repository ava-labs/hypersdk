// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

var _ state.Mutable = (*InMemoryStore)(nil)

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

type ActionTest struct {
	Action Action

	Rules     Rules
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

func (suite *ActionTestSuite) Run(t *testing.T) {
	for testName := range suite.Tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)
			test := suite.Tests[testName]

			output, err := test.Action.Execute(context.TODO(), test.Rules, test.State, test.Timestamp, test.Actor, test.ActionID)

			require.ErrorIs(err, test.ExpectedErr)
			require.Equal(output, test.ExpectedOutputs)
		})
	}
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

var _ chain.PendingView = (*InMemoryStore)(nil)

// InMemoryStore is an in-memory implementation of `state.Mutable`
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

func (*InMemoryStore) Rollback(context.Context, int) {
	// TODO implement me
	panic("implement me")
}

func (*InMemoryStore) OpIndex() int {
	// TODO implement me
	panic("implement me")
}

func (*InMemoryStore) Commit() {
	// TODO implement me
	panic("implement me")
}

// ActionTest is a single parameterized test. It calls Execute on the action with the passed parameters
// and checks that all assertions pass.
type ActionTest[T chain.PendingView] struct {
	Name string

	Action chain.Action[T]

	Rules     chain.Rules
	View      T
	Timestamp int64
	Actor     codec.Address
	ActionID  ids.ID

	ExpectedOutputs codec.Typed
	ExpectedErr     error

	Assertion func(context.Context, *testing.T, T)
}

// Run executes the [ActionTest] and make sure all assertions pass.
func (test *ActionTest[T]) Run(ctx context.Context, t *testing.T) {
	t.Run(test.Name, func(t *testing.T) {
		require := require.New(t)

		output, err := test.Action.Execute(
			ctx,
			test.Rules,
			test.View,
			test.Timestamp,
			test.Actor,
			test.ActionID,
		)

		require.ErrorIs(err, test.ExpectedErr)
		require.Equal(output, test.ExpectedOutputs)

		if test.Assertion != nil {
			test.Assertion(ctx, t, test.View)
		}
	})
}

// ActionBenchmark is a parameterized benchmark. It calls Execute on the action with the passed parameters
// and checks that all assertions pass. To avoid using shared state between runs, a new
// state is created for each iteration using the provided `CreateState` function.
type ActionBenchmark[T chain.PendingView] struct {
	Name   string
	Action chain.Action[T]

	Rules     chain.Rules
	NewViewF  func() T
	Timestamp int64
	Actor     codec.Address
	ActionID  ids.ID

	ExpectedOutputs []codec.Typed
	ExpectedErr     error

	Assertion func(context.Context, *testing.B, T)
}

// Run executes the [ActionBenchmark] and make sure all the benchmark assertions pass.
func (test *ActionBenchmark[T]) Run(ctx context.Context, b *testing.B) {
	require := require.New(b)

	// create a slice of b.N states
	runtimes := make([]T, b.N)
	for i := 0; i < b.N; i++ {
		runtimes[i] = test.NewViewF()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		output, err := test.Action.Execute(ctx, test.Rules, runtimes[i], test.Timestamp, test.Actor, test.ActionID)
		require.NoError(err)
		require.Equal(output, test.ExpectedOutputs)
	}

	b.StopTimer()
	// check assertions
	if test.Assertion != nil {
		for i := 0; i < b.N; i++ {
			test.Assertion(ctx, b, runtimes[i])
		}
	}
}

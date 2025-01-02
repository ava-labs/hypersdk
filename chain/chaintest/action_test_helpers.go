// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

var (
	_ chain.Action  = (*TestAction)(nil)
	_ state.Mutable = (*InMemoryStore)(nil)
)

var errTestActionExecute = errors.New("test action execute error")

type TestAction struct {
	NumComputeUnits    uint64     `serialize:"true" json:"computeUnits"`
	SpecifiedStateKeys state.Keys `serialize:"true" json:"specifiedStateKeys"`
	ReadKeys           [][]byte   `serialize:"true" json:"reads"`
	WriteKeys          [][]byte   `serialize:"true" json:"writeKeys"`
	WriteValues        [][]byte   `serialize:"true" json:"writeValues"`
	ExecuteErr         bool       `serialize:"true" json:"executeErr"`
	Nonce              uint64     `serialize:"true" json:"nonce"`
}

func (*TestAction) GetTypeID() uint8 {
	return 0
}

func (t *TestAction) ComputeUnits(_ chain.Rules) uint64 {
	return t.NumComputeUnits
}

func (t *TestAction) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	return t.SpecifiedStateKeys
}

func (t *TestAction) Execute(ctx context.Context, _ chain.Rules, state state.Mutable, _ int64, _ codec.Address, _ ids.ID) (codec.Typed, error) {
	if t.ExecuteErr {
		return nil, errTestActionExecute
	}
	for _, key := range t.ReadKeys {
		if _, err := state.GetValue(ctx, key); err != nil {
			return nil, err
		}
	}
	if len(t.WriteKeys) != len(t.WriteValues) {
		return nil, fmt.Errorf("mismatch write keys/values (%d != %d)", len(t.WriteKeys), len(t.WriteValues))
	}
	for i, key := range t.WriteKeys {
		if err := state.Insert(ctx, key, t.WriteValues[i]); err != nil {
			return nil, err
		}
	}
	return &TestOutput{}, nil
}

func (*TestAction) ValidRange(_ chain.Rules) (start int64, end int64) {
	return -1, -1
}

type TestOutput struct{}

func (*TestOutput) GetTypeID() uint8 {
	return 0
}

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

// ActionTest is a single parameterized test. It calls Execute on the action with the passed parameters
// and checks that all assertions pass.
type ActionTest struct {
	Name string

	Action chain.Action

	Rules     chain.Rules
	State     state.Mutable
	Timestamp int64
	Actor     codec.Address
	ActionID  ids.ID

	ExpectedOutputs codec.Typed
	ExpectedErr     error

	Assertion func(context.Context, *testing.T, state.Mutable)
}

// Run executes the [ActionTest] and make sure all assertions pass.
func (test *ActionTest) Run(ctx context.Context, t *testing.T) {
	t.Run(test.Name, func(t *testing.T) {
		require := require.New(t)

		output, err := test.Action.Execute(ctx, test.Rules, test.State, test.Timestamp, test.Actor, test.ActionID)

		require.ErrorIs(err, test.ExpectedErr)
		require.Equal(test.ExpectedOutputs, output)

		if test.Assertion != nil {
			test.Assertion(ctx, t, test.State)
		}
	})
}

// ActionBenchmark is a parameterized benchmark. It calls Execute on the action with the passed parameters
// and checks that all assertions pass. To avoid using shared state between runs, a new
// state is created for each iteration using the provided `CreateState` function.
type ActionBenchmark struct {
	Name   string
	Action chain.Action

	Rules       chain.Rules
	CreateState func() state.Mutable
	Timestamp   int64
	Actor       codec.Address
	ActionID    ids.ID

	ExpectedOutput codec.Typed
	ExpectedErr    error

	Assertion func(context.Context, *testing.B, state.Mutable)
}

// Run executes the [ActionBenchmark] and make sure all the benchmark assertions pass.
func (test *ActionBenchmark) Run(ctx context.Context, b *testing.B) {
	require := require.New(b)

	// create a slice of b.N states
	states := make([]state.Mutable, b.N)
	for i := 0; i < b.N; i++ {
		states[i] = test.CreateState()
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		output, err := test.Action.Execute(ctx, test.Rules, states[i], test.Timestamp, test.Actor, test.ActionID)
		require.NoError(err)
		require.Equal(test.ExpectedOutput, output)
	}

	b.StopTimer()
	// check assertions
	if test.Assertion != nil {
		for i := 0; i < b.N; i++ {
			test.Assertion(ctx, b, states[i])
		}
	}
}

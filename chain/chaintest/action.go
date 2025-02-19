// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

var _ chain.Action = (*TestAction)(nil)

var ErrTestActionExecute = errors.New("test action execute error")

type TestAction struct {
	NumComputeUnits              uint64              `canoto:"fint64,1" serialize:"true" json:"computeUnits"`
	SpecifiedStateKeys           []string            `canoto:"repeated string,2" serialize:"true" json:"specifiedStateKeys"`
	SpecifiedStateKeyPermissions []state.Permissions `canoto:"repeated int,3" serialize:"true" json:"specifiedStateKeyPermissions"`
	ReadKeys                     [][]byte            `canoto:"repeated bytes,4" serialize:"true" json:"reads"`
	WriteKeys                    [][]byte            `canoto:"repeated bytes,5" serialize:"true" json:"writeKeys"`
	WriteValues                  [][]byte            `canoto:"repeated bytes,6" serialize:"true" json:"writeValues"`
	ExecuteErr                   bool                `canoto:"bool,7" serialize:"true" json:"executeErr"`
	Nonce                        uint64              `canoto:"fint64,8" serialize:"true" json:"nonce"`
	Start                        int64               `canoto:"sint,9" serialize:"true" json:"start"`
	End                          int64               `canoto:"sint,10" serialize:"true" json:"end"`

	canotoData canotoData_TestAction
}

func (*TestAction) GetTypeID() uint8 {
	return 0
}

func (t *TestAction) Bytes() []byte {
	return t.MarshalCanoto()
}

func (t *TestAction) ComputeUnits(_ chain.Rules) uint64 {
	return t.NumComputeUnits
}

func (t *TestAction) StateKeys(_ codec.Address, _ ids.ID) state.Keys {
	stateKeys := make(state.Keys)
	for i, key := range t.SpecifiedStateKeys {
		// Avoid a panic on invalid state key permissions and return the state keys
		// gathered thus far.
		// Possible alternative behavior would be to populate remaining keys with state.None
		// as the permission.
		if i >= len(t.SpecifiedStateKeyPermissions) {
			break
		}
		stateKeys[key] = t.SpecifiedStateKeyPermissions[i]
	}
	return stateKeys
}

func (t *TestAction) Execute(ctx context.Context, _ chain.Rules, state state.Mutable, _ int64, _ codec.Address, _ ids.ID) (codec.Typed, error) {
	if t.ExecuteErr {
		return nil, ErrTestActionExecute
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

// ValidRange returns the start/end fields of the action unless 0 is specified.
// If 0 is specified, return -1 for always valid, which is a more useful default value.
func (t *TestAction) ValidRange(_ chain.Rules) (int64, int64) {
	start := t.Start
	end := t.End
	if start == 0 {
		start = -1
	}
	if end == 0 {
		end = -1
	}
	return start, end
}

type TestOutput struct{}

func (*TestOutput) GetTypeID() uint8 {
	return 0
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

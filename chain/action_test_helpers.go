// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import (
	"context"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

type ActionTest struct {
	Action Action

	Rules     Rules
	State     state.Mutable
	Timestamp int64
	Actor     codec.Address
	ActionID  ids.ID

	ExpectedOutputs   [][]byte
	ExpectedErr       error
	ContainsErrString string
}

type ActionTestSuite struct {
	Tests map[string]ActionTest

	Setup    func()
	Teardown func()
}

func (suite *ActionTestSuite) Run(t *testing.T) {
	if suite.Setup != nil {
		suite.Setup()
	}

	for testName := range suite.Tests {
		t.Run(testName, func(t *testing.T) {
			require := require.New(t)
			test := suite.Tests[testName]

			// Ensure that only one of outputs or err is set
			if test.ExpectedOutputs != nil && (test.ExpectedErr != nil || len(test.ContainsErrString) > 0) {
				t.Fatalf("ExpectedOutputs and ExpectedErr are set in test %s", testName)
			}
			if test.ExpectedOutputs == nil && test.ExpectedErr == nil && len(test.ContainsErrString) == 0 {
				t.Fatalf("neither ExpectedOutputs nor ExpectedErr is set in test %s", testName)
			}

			output, err := test.Action.Execute(context.TODO(), test.Rules, test.State, test.Timestamp, test.Actor, test.ActionID)

			if err != nil {
				if test.ExpectedErr == nil && len(test.ContainsErrString) == 0 {
					t.Fatalf("got unexpected error: %w", err)
				}

				if test.ExpectedErr != nil {
					require.Equal(err, test.ExpectedErr)
				}
				if len(test.ContainsErrString) > 0 {
					require.Contains(err.Error(), test.ContainsErrString)
				}
			}

			require.Equal(output, test.ExpectedOutputs)
		})
	}

	if suite.Teardown != nil {
		suite.Teardown()
	}
}

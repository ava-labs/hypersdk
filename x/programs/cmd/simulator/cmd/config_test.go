// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateAssertion(t *testing.T) {
	require := require.New(t)
	tests := []struct {
		actual    int64
		assertion Require
		expected  bool
		wantErr   bool
	}{
		{5, Require{ResultAssertion{Operator: string(NumericGt), Value: "3"}}, true, false},
		{5, Require{ResultAssertion{Operator: string(NumericLt), Value: "10"}}, true, false},
		{5, Require{ResultAssertion{Operator: string(NumericEq), Value: "5"}}, true, false},
		{5, Require{ResultAssertion{Operator: string(NumericNe), Value: "3"}}, true, false},
		{5, Require{ResultAssertion{Operator: string(NumericGt), Value: "10"}}, false, false},
		{5, Require{ResultAssertion{Operator: string(NumericLt), Value: "2"}}, false, false},
		{5, Require{ResultAssertion{Operator: string(NumericGe), Value: "5"}}, true, false},
		{5, Require{ResultAssertion{Operator: string(NumericGe), Value: "1"}}, true, false},
		{5, Require{}, false, true},
	}

	for _, tt := range tests {
		result, err := validateAssertion(tt.actual, &tt.assertion)
		if tt.wantErr {
			require.Error(err)
		} else {
			require.NoError(err)
		}
		require.Equal(tt.expected, result)
	}
}

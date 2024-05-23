// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"strconv"
	"testing"

	"github.com/near/borsh-go"
	"github.com/stretchr/testify/require"
)

func TestValidateAssertion(t *testing.T) {
	require := require.New(t)
	tests := []struct {
		actual    int64
		assertion Require
		expected  bool
		wantErr   error
	}{
		{5, Require{ResultAssertion{Operator: string(NumericGt), Value: "3"}}, true, nil},
		{5, Require{ResultAssertion{Operator: string(NumericLt), Value: "10"}}, true, nil},
		{5, Require{ResultAssertion{Operator: string(NumericEq), Value: "5"}}, true, nil},
		{5, Require{ResultAssertion{Operator: string(NumericNe), Value: "3"}}, true, nil},
		{5, Require{ResultAssertion{Operator: string(NumericGt), Value: "10"}}, false, nil},
		{5, Require{ResultAssertion{Operator: string(NumericLt), Value: "2"}}, false, nil},
		{5, Require{ResultAssertion{Operator: string(NumericGe), Value: "5"}}, true, nil},
		{5, Require{ResultAssertion{Operator: string(NumericGe), Value: "1"}}, true, nil},
		{5, Require{}, false, strconv.ErrSyntax},
	}

	for _, tt := range tests {
		actual, err := borsh.Serialize(tt.actual)
		require.NoError(err)
		assertion := tt.assertion
		result, err := validateAssertion(actual, &assertion)
		if tt.wantErr != nil {
			require.ErrorIs(err, tt.wantErr)
		} else {
			require.NoError(err)
		}
		require.Equal(tt.expected, result)
	}
}

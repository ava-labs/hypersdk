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
		name      string
		actual    int64
		assertion *Require
		expected  bool
		wantErr   error
	}{
		{"IsGreaterThan", 5, &Require{ResultAssertion{Operator: string(NumericGt), Value: "3"}}, true, nil},
		{"IsNotGreaterThan", 5, &Require{ResultAssertion{Operator: string(NumericGt), Value: "10"}}, false, nil},
		{"IsLessThan", 5, &Require{ResultAssertion{Operator: string(NumericLt), Value: "10"}}, true, nil},
		{"IsNotLessThan", 5, &Require{ResultAssertion{Operator: string(NumericLt), Value: "2"}}, false, nil},
		{"IsEqualTo", 5, &Require{ResultAssertion{Operator: string(NumericEq), Value: "5"}}, true, nil},
		{"IsNotEqual", 5, &Require{ResultAssertion{Operator: string(NumericNe), Value: "3"}}, true, nil},
		{"IsGreaterThanOrEqualToSame", 5, &Require{ResultAssertion{Operator: string(NumericGe), Value: "5"}}, true, nil},
		{"IsGreaterThanOrEqualToSmaller", 5, &Require{ResultAssertion{Operator: string(NumericGe), Value: "1"}}, true, nil},
		{"ParseNothingFails", 5, &Require{}, false, strconv.ErrSyntax},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(*testing.T) {
			actual, err := borsh.Serialize(tt.actual)
			require.NoError(err)
			result, err := validateAssertion(actual, tt.assertion)
			if tt.wantErr != nil {
				require.ErrorIs(err, tt.wantErr)
			} else {
				require.NoError(err)
			}
			require.Equal(tt.expected, result)
		})
	}
}

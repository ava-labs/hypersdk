// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import "testing"

func TestValidateAssertion(t *testing.T) {
	tests := []struct {
		actual    uint64
		assertion ResultAssertion
		expected  bool
	}{
		{5, ResultAssertion{Operator: string(NumericGt), Value: "3"}, true},
		{5, ResultAssertion{Operator: string(NumericLt), Value: "10"}, true},
		{5, ResultAssertion{Operator: string(NumericEq), Value: "5"}, true},
		{5, ResultAssertion{Operator: string(NumericNe), Value: "3"}, true},
		{5, ResultAssertion{Operator: string(NumericGt), Value: "10"}, false},
		{5, ResultAssertion{Operator: string(NumericLt), Value: "2"}, false},
		{5, ResultAssertion{Operator: string(NumericGe), Value: "5"}, true},
		{5, ResultAssertion{Operator: string(NumericGe), Value: "1"}, true},
	}

	for _, tt := range tests {
		result := validateAssertion(tt.actual, &tt.assertion)
		if result != tt.expected {
			t.Errorf("validateAssertion(%d, %+v) = %v; want %v", tt.actual, tt.assertion, result, tt.expected)
		}
	}
}

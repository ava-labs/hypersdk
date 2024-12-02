// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLargestSet(t *testing.T) {
	r := require.New(t)

	testCases := []struct {
		name            string
		dim             []Dimensions
		limit           Dimensions
		expectedIndices []uint64
		expectedDim     Dimensions
	}{
		{
			name: "case1",
			dim: []Dimensions{
				{1, 0, 0, 0, 0},
				{2, 0, 0, 0, 0},
				{3, 0, 0, 0, 0},
				{4, 0, 0, 0, 0},
				{5, 0, 0, 0, 0},
			},
			limit:           Dimensions{4, 0, 0, 0, 0},
			expectedIndices: []uint64{0, 1},
			expectedDim:     Dimensions{3, 0, 0, 0, 0},
		},
		{
			name: "case2",
			dim: []Dimensions{
				{1, 0, 0, 0, 0},
				{2, 0, 0, 0, 0},
				{3, 0, 0, 0, 0},
				{4, 0, 0, 0, 0},
				{5, 0, 0, 0, 0},
			},
			limit:           Dimensions{6, 0, 0, 0, 0},
			expectedIndices: []uint64{0, 1, 2},
			expectedDim:     Dimensions{6, 0, 0, 0, 0},
		},
		{
			name: "case3",
			dim: []Dimensions{
				{1, 0, 0, 0, 0},
				{4, 0, 0, 0, 0},
				{2, 0, 0, 0, 0},
				{5, 0, 0, 0, 0},
				{3, 0, 0, 0, 0},
			},
			limit:           Dimensions{6, 0, 0, 0, 0},
			expectedIndices: []uint64{0, 2, 4},
			expectedDim:     Dimensions{6, 0, 0, 0, 0},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			indices, acc := LargestSet(testCase.dim, testCase.limit)
			r.Equal(testCase.expectedIndices, indices)
			r.Equal(testCase.expectedDim, acc)
		})
	}
}

func BenchmarkLargestSet(b *testing.B) {
	r := require.New(b)

	for n := 0; n < b.N; n++ {
		dimensions := make([]Dimensions, 20)

		for i := range dimensions {
			d := uint64(i)
			dimensions[i] = Dimensions{d % 1000, (d + 200) % 1000, (d + 400) % 1000, (d + 600) % 1000, (d + 800) % 1000}
		}

		limit := Dimensions{50000, 50000, 50000, 50000, 50000}

		indices, _ := LargestSet(dimensions, limit)
		r.NotEqual(len(indices), 0)
	}
}

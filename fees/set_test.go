// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"testing"

	"github.com/stretchr/testify/require"

	rand "math/rand/v2"
)

func TestLargestSet(t *testing.T) {
	tests := []struct {
		name            string
		dim             []Dimensions
		limit           Dimensions
		expectedIndices []uint64
		expectedDim     Dimensions
	}{
		{
			name: "trivial",
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
			name: "trivial2",
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
			name: "ordering",
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
		{
			name: "multi-dimension",
			dim: []Dimensions{
				{1, 0, 0, 0, 0},
				{0, 2, 0, 0, 0},
				{0, 0, 3, 0, 0},
				{0, 0, 0, 4, 0},
				{0, 0, 0, 0, 5},
			},
			limit:           Dimensions{6, 6, 6, 3, 3},
			expectedIndices: []uint64{0, 1, 2},
			expectedDim:     Dimensions{1, 2, 3, 0, 0},
		},
		{
			name: "multi-dimension2",
			dim: []Dimensions{
				{5, 1, 1, 1, 1},
				{1, 5, 1, 1, 1},
				{1, 1, 5, 1, 1},
				{1, 1, 1, 5, 1},
				{1, 1, 1, 1, 5},
			},
			limit:           Dimensions{7, 7, 7, 7, 7},
			expectedIndices: []uint64{0, 1, 2},
			expectedDim:     Dimensions{7, 7, 7, 3, 3},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			r := require.New(t)
			indices, acc := LargestSet(testCase.dim, testCase.limit)
			r.Equal(testCase.expectedIndices, indices)
			r.Equal(testCase.expectedDim, acc)
		})
	}
}

func BenchmarkLargestSet(b *testing.B) {
	r := require.New(b)
	rnd := rand.NewChaCha8([32]byte{0})

	// nDimCount number of dimensions ( i.e. transaction count )
	const nDimCount int = 20000
	// nDimLimit what is the limit for each of the dimensions
	const nDimLimit uint64 = 50000
	// nDimRange when generating the random dimensions, what range should each of the elements be.
	// this need to be notably smaller than nDimLimit so that we can fit multiple transaction into
	// a single set.
	const nDimRange uint64 = 1000
	for n := 0; n < b.N; n++ {
		dimensions := make([]Dimensions, nDimCount)
		for i := range dimensions {
			for j := 0; j < FeeDimensions; j++ {
				dimensions[i][j] = rnd.Uint64() % nDimRange
			}
		}

		limit := Dimensions{nDimLimit, nDimLimit, nDimLimit, nDimLimit, nDimLimit}
		b.StartTimer()
		indices, _ := LargestSet(dimensions, limit)
		b.StopTimer()
		r.NotEmpty(indices)
	}
}

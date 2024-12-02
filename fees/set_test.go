// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetIncludeDim(t *testing.T) {
	r := require.New(t)

	dimensions := []Dimensions{
		{1, 0, 0, 0, 0},
		{2, 0, 0, 0, 0},
		{3, 0, 0, 0, 0},
		{4, 0, 0, 0, 0},
		{5, 0, 0, 0, 0},
	}
	s := setState{
		pendingDimentions: []uint64{0, 1, 2, 3, 4},
	}
	outState := s.includeDimension(1, dimensions)
	r.Len(outState.includedDimentions, 1)
	r.Len(outState.pendingDimentions, 4)
}

func TestLargestSet(t *testing.T) {
	r := require.New(t)

	dimensions := []Dimensions{
		{1, 0, 0, 0, 0},
		{2, 0, 0, 0, 0},
		{3, 0, 0, 0, 0},
		{4, 0, 0, 0, 0},
		{5, 0, 0, 0, 0},
	}
	limit := Dimensions{4, 0, 0, 0, 0}
	indices, acc := LargestSet(dimensions, limit)
	r.Len(indices, 2)
	r.Equal(Dimensions{3, 0, 0, 0, 0}, acc)
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

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"sort"
)

// LargestSet takes a slice of dimensions and a dimensions limit, and find the
// largests set of dimensions that would fit within the provided limit. The return
// slice is the list of indices relative to the provided [dimensions] slice.
// note that the solution of the largest set is not
// deterministic.
func LargestSet(dimensions []Dimensions, limit Dimensions) ([]uint64, Dimensions) {
	outIndices := make([]uint64, len(dimensions))
	weights := make([]float64, len(dimensions))
	for i := range dimensions {
		outIndices[i] = uint64(i)
		size := float64(0)
		for k := 0; k < FeeDimensions; k++ {
			w := float64(0)
			if limit[k] > 0 {
				w = float64(dimensions[i][k]) / float64(limit[k])
			}
			size += w * w
		}
		weights[i] = size
	}
	sort.SliceStable(outIndices, func(i, j int) bool {
		return weights[outIndices[i]] < weights[outIndices[j]]
	})
	// find where we pass the limit threshold.
	var accumulator Dimensions
	var err error
	for i, j := range outIndices {
		if !accumulator.CanAdd(dimensions[j], limit) {
			return outIndices[:i], accumulator
		}
		accumulator, err = accumulator.AddDimentions(dimensions[j])
		if err != nil {
			return []uint64{}, Dimensions{}
		}
	}
	return outIndices, accumulator
}

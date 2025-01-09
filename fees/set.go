// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"math/big"
	"sort"
)

// LargestSet takes a slice of dimensions and a dimensions limit, and find the
// largest set of dimensions that would fit within the provided limit. The return
// slice is the list of indices relative to the provided [dimensions] slice.
// note that the solution of the largest set is not
// deterministic ( by the nature of the problem, there could be multiple "correct"
// and different results )
//
// The algorithm used here is the following:
//  1. Scaling: Each dimension is scaled relative to its respective limit
//     to ensure fair comparison. This step normalizes the dimensions,
//     preventing larger dimensions from dominating the selection process.
//  2. Vector Sizing: Each dimension is treated as a vector, and its
//     magnitude is calculated. This metric is used to assess the
//     relative significance of each dimension.
//  3. Greedy Selection: Dimensions are iteratively selected based on
//     their scaled magnitudes, adding them to the subset until the size
//     limit is reached. The greedy approach prioritizes the most significant
//     dimensions, maximizing the overall subset size.
//
// Implementation notes:
// * Precision: To mitigate potential precision issues arising from
// small-scale dimensions, the function employs a scaling factor to
// amplify values before normalization. This ensures accurate calculations
// even when dealing with very small numbers.
// * Efficiency: The squared magnitude calculation is used as a proxy
// for Euclidean distance, optimizing performance without sacrificing
// accuracy for relative comparisons. This optimization avoids the
// computationally expensive square root operation.
func LargestSet(dimensions []Dimensions, limit Dimensions) ([]uint64, Dimensions) {
	outIndices := make([]uint64, len(dimensions))
	weights := make([]*big.Int, len(dimensions))
	shift := big.NewInt(65536)
	for i := range dimensions {
		outIndices[i] = uint64(i)
		size := big.NewInt(0)
		for k := 0; k < FeeDimensions; k++ {
			w := big.NewInt(0)
			if limit[k] > 0 {
				// calculate w = (shift * dimensions[i][k] ^ 2) / (limit[k] ^ 2)
				d := big.NewInt(int64(dimensions[i][k]))
				d.Mul(d, d)
				d.Mul(d, shift)

				b := big.NewInt(int64(limit[k]))
				b = b.Mul(b, b)

				w = w.Div(d, b)
			}
			size = size.Add(size, w)
		}
		weights[i] = size
	}

	sort.SliceStable(outIndices, func(i, j int) bool {
		return weights[outIndices[i]].Cmp(weights[outIndices[j]]) < 0
	})

	// find where we pass the limit threshold.
	var accumulator Dimensions
	var err error
	for i := 0; i < len(outIndices); i++ {
		dim := dimensions[outIndices[i]]
		if !accumulator.CanAdd(dim, limit) {
			// mark this index so we can remove it later on.
			outIndices[i] = uint64(len(dimensions))
			continue
		}
		accumulator, err = Add(accumulator, dim)
		if err != nil {
			return []uint64{}, Dimensions{}
		}
	}
	// remove all unwanted indices from the array.
	j := 0
	for i := 0; i < len(outIndices)-j; i++ {
		if outIndices[i] == uint64(len(dimensions)) {
			j++
			i--
			continue
		}
		outIndices[i] = outIndices[i+j]
	}
	outIndices = outIndices[:len(outIndices)-j]
	return outIndices, accumulator
}

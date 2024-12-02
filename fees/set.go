// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"math/big"
	"sort"
)

// LargestSet takes a slice of dimensions and a dimensions limit, and find the
// largests set of dimensions that would fit within the provided limit. The return
// slice is the list of indices relative to the provided [dimensions] slice.
// note that the solution of the largest set is not
// deterministic.
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
			outIndices = append(outIndices[:i], outIndices[i+1:]...)
			i--
			continue
		}
		accumulator, err = accumulator.AddDimentions(dim)
		if err != nil {
			return []uint64{}, Dimensions{}
		}
	}
	return outIndices, accumulator
}

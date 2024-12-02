// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fees

import (
	"crypto/sha256"
	"encoding/binary"
	"slices"
)

const uint64Size = int(8)

type (
	stateKey [sha256.Size]byte
	setState struct {
		accumulatedDimentions Dimensions
		pendingDimentions     []uint64
		includedDimentions    []uint64
	}
)

func (s *setState) stateKey() stateKey {
	o := make([]byte, len(s.includedDimentions)*uint64Size)
	for i, d := range s.includedDimentions {
		binary.BigEndian.PutUint64(o[i*uint64Size:], d)
	}
	return sha256.Sum256(o)
}

func (s *setState) includeDimension(k int, dimensions []Dimensions) *setState {
	dim := s.pendingDimentions[k]
	newAcc, err := s.accumulatedDimentions.AddDimentions(dimensions[dim])
	if err != nil {
		return nil
	}
	newSet := &setState{
		accumulatedDimentions: newAcc,
		pendingDimentions:     make([]uint64, k, len(s.pendingDimentions)-1),
		includedDimentions:    make([]uint64, len(s.includedDimentions)+1),
	}
	i, _ := slices.BinarySearch(s.includedDimentions, dim)
	copy(newSet.includedDimentions, s.includedDimentions[:i])
	newSet.includedDimentions[i] = dim
	copy(newSet.includedDimentions[i+1:], s.includedDimentions[i:])

	copy(newSet.pendingDimentions, s.pendingDimentions[:k])
	newSet.pendingDimentions = append(newSet.pendingDimentions, s.pendingDimentions[k+1:]...)
	return newSet
}

// LargestSet takes a slice of dimensions and a dimensions limit, and find the
// largests set of dimensions that would fit within the provided limit. The return
// slice is the list of indices relative to the provided [dimensions] slice.
// note that the solution of the largest set is not
// deterministic.
func LargestSet(dimensions []Dimensions, limit Dimensions) ([]uint64, Dimensions) {
	initialState := setState{
		pendingDimentions: make([]uint64, len(dimensions)),
	}
	for i := range dimensions {
		initialState.pendingDimentions[i] = uint64(i)
	}
	options := []*setState{
		&initialState,
	}
	progress := true
	for progress {
		progress = false
		nextOptions := []*setState{}
		nextOptionsMap := map[stateKey]bool{}
		// iterate on the options and try to make progress on each one of them.
		for _, opt := range options {
			for i, dimIdx := range opt.pendingDimentions {
				if !opt.accumulatedDimentions.CanAdd(dimensions[dimIdx], limit) {
					continue
				}
				newDim := opt.includeDimension(i, dimensions)
				newKey := newDim.stateKey()
				if nextOptionsMap[newKey] {
					continue
				}
				nextOptionsMap[newKey] = true
				nextOptions = append(nextOptions, newDim)
				progress = true
			}
		}
		if progress {
			options = nextOptions
		}
	}
	// return any of the remaining sets.
	for _, v := range options {
		return v.includedDimentions, v.accumulatedDimentions
	}
	return []uint64{}, Dimensions{}
}

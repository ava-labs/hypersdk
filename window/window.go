// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package window

import (
	"encoding/binary"

	"github.com/ava-labs/avalanchego/utils/math"

	"github.com/ava-labs/hypersdk/consts"
)

const (
	WindowSize      = 10
	WindowSliceSize = WindowSize * consts.Uint64Len
)

type Window [WindowSliceSize]byte

// Roll rolls the uint64s within [consumptionWindow] over by [roll] places.
// For example, if there are 4 uint64 encoded in a 32 byte slice, rollWindow would
// have the following effect:
// Original:
// [1, 2, 3, 4]
// Roll = 0
// [1, 2, 3, 4]
// Roll = 1
// [2, 3, 4, 0]
// Roll = 2
// [3, 4, 0, 0]
// Roll = 3
// [4, 0, 0, 0]
// Roll >= 4
// [0, 0, 0, 0]
// Assumes that [roll] is greater than or equal to 0
func Roll(w Window, roll int) (Window, error) {
	// Note: make allocates a zeroed array, so we are guaranteed
	// that what we do not copy into, will be set to 0
	res := [WindowSliceSize]byte{}
	bound := roll * consts.Uint64Len
	if bound > WindowSliceSize {
		return res, nil
	}
	copy(res[:], w[roll*consts.Uint64Len:])
	return res, nil
}

// Sum sums [numUint64s] encoded in [window]. Assumes that the length of [window]
// is sufficient to contain [numUint64s] or else this function panics.
// If an overflow occurs, while summing the contents, the maximum uint64 value is returned.
func Sum(w Window) uint64 {
	var (
		sum      uint64
		overflow error
	)
	for i := 0; i < 10; i++ {
		// If an overflow occurs while summing the elements of the window, return the maximum
		// uint64 value immediately.
		sum, overflow = math.Add64(sum, binary.BigEndian.Uint64(w[consts.Uint64Len*i:]))
		if overflow != nil {
			return consts.MaxUint64
		}
	}
	return sum
}

// Update adds [unitsConsumed] in at index within [window].
// Assumes that [index] has already been validated.
// If an overflow occurs, the maximum uint64 value is used.
func Update(w *Window, start int, unitsConsumed uint64) {
	prevUnitsConsumed := binary.BigEndian.Uint64(w[start:])

	totalUnitsConsumed, overflow := math.Add64(prevUnitsConsumed, unitsConsumed)
	if overflow != nil {
		totalUnitsConsumed = consts.MaxUint64
	}
	binary.BigEndian.PutUint64(w[start:], totalUnitsConsumed)
}

func Last(w *Window) uint64 {
	return binary.BigEndian.Uint64(w[WindowSliceSize-consts.Uint64Len:])
}

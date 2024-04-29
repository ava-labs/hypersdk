// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package window

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/consts"
)

func testRollup(t *testing.T, uint64s []uint64, roll int) {
	require := require.New(t)
	slice := [WindowSliceSize]byte{}
	numUint64s := len(uint64s)
	for i := 0; i < numUint64s; i++ {
		binary.BigEndian.PutUint64(slice[8*i:], uint64s[i])
	}

	newSlice, err := Roll(slice, roll)
	require.NoError(err)
	// numCopies is the number of uint64s that should have been copied over from the previous
	// slice as opposed to being left empty.
	numCopies := numUint64s - roll
	for i := 0; i < numUint64s; i++ {
		// Extract the uint64 value that is encoded at position [i] in [newSlice]
		num := binary.BigEndian.Uint64(newSlice[8*i:])
		// If the current index is past the point where we should have copied the value
		// over from the previous slice, assert that the value encoded in [newSlice]
		// is 0
		if i >= numCopies {
			require.Zero(num, "position=%d", i)
		} else {
			// Otherwise, check that the value was copied over correctly
			prevIndex := i + roll
			prevNum := uint64s[prevIndex]
			require.Equal(prevNum, num, "position=%d", i)
		}
	}
}

func TestRollupWindow(t *testing.T) {
	type test struct {
		uint64s []uint64
		roll    int
	}

	tests := []test{
		{
			[]uint64{1, 2, 3, 4},
			0,
		},
		{
			[]uint64{1, 2, 3, 4},
			1,
		},
		{
			[]uint64{1, 2, 3, 4},
			2,
		},
		{
			[]uint64{1, 2, 3, 4},
			3,
		},
		{
			[]uint64{1, 2, 3, 4},
			4,
		},
		{
			[]uint64{1, 2, 3, 4},
			5,
		},
		{
			[]uint64{121, 232, 432},
			2,
		},
	}

	for _, test := range tests {
		testRollup(t, test.uint64s, test.roll)
	}
}

func TestUint64Window(t *testing.T) {
	require := require.New(t)
	uint64s := []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	sumUint64s := uint64(0)
	uint64Window := Window{}
	for i, uint64 := range uint64s {
		sumUint64s += uint64
		binary.BigEndian.PutUint64(uint64Window[i*8:], uint64)
	}

	sum := Sum(uint64Window)
	require.Equal(sumUint64s, sum)

	for i := 0; i < 10; i++ {
		iu64 := uint64(i)
		Update(&uint64Window, i*8, iu64)
		sum = Sum(uint64Window)
		sumUint64s += iu64

		require.Equal(sumUint64s, sum, "i=%d", i)
	}
}

func TestUint64WindowOverflow(t *testing.T) {
	require := require.New(t)
	uint64s := []uint64{0, 0, 0, 0, 0, 0, 0, 0, 2, consts.MaxUint64 - 1}
	uint64Window := Window{}
	for i, uint64 := range uint64s {
		binary.BigEndian.PutUint64(uint64Window[i*8:], uint64)
	}

	sum := Sum(uint64Window)
	require.Equal(consts.MaxUint64, sum)

	for i := 0; i < 10; i++ {
		Update(&uint64Window, i*8, uint64(i))
		sum = Sum(uint64Window)

		require.Equal(consts.MaxUint64, sum)
	}
}

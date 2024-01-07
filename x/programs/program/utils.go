// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import "math"

// EnsureUint64ToInt32 returns true if [v] (uint32) can be safely converted to an int32.
func EnsureUint32ToInt32(v uint32) bool {
	return v <= math.MaxInt32
}

// EnsureIntToInt32 returns true if [v] (int) can be safely converted to an int32.
func EnsureIntToInt32(v int) bool {
	return v >= math.MinInt32 && v <= math.MaxInt32
}

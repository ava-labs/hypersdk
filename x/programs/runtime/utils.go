// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"math"
)

func EnsureInt64ToInt32(v int64) bool {
	return v >= math.MinInt32 && v <= math.MaxInt32
}

func EnsureUint32ToInt32(v uint32) bool {
	return v <= math.MaxInt32
}

func EnsureIntToInt32(v int) bool {
	return v >= math.MinInt32 && v <= math.MaxInt32
}

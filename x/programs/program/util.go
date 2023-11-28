// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import "math"

func EnsureInt64ToInt32(v int64) bool {
	return v >= math.MinInt32 && v <= math.MaxInt32
}

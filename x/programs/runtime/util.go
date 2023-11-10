package runtime

import "math"

func EnsureInt64ToInt32(v int64) bool {
	return v >= math.MinInt32 && v <= math.MaxInt32
}

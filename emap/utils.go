// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.
package emap

// timeModulus is used to modulo all the timestamps passed to emap to avoid an
// explosion of items in internal structs (we only need second-level precision).
const timeModulus = 1000 // ms -> s

func reducePrecision(t int64) int64 {
	return t - t%timeModulus
}

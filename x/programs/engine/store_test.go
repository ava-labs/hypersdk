// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	"testing"
)

// go test -v -benchmem -run=^$ -bench ^BenchmarkNewStore$ github.com/ava-labs/hypersdk/x/programs/engine -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkNewStore(b *testing.B) {
	eng := New(NewConfig())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewStore(eng, NewStoreConfig())
	}
}

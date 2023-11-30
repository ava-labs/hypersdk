// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// go test -v -benchmem -run=^$ -bench ^BenchmarkNewStore$ github.com/ava-labs/hypersdk/x/programs/engine -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkNewStore(b *testing.B) {
	require := require.New(b)
	cfg, err := NewConfigBuilder().Build()
	require.NoError(err)
	eng, err := New(cfg)
	require.NoError(err)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewStore(eng)
		require.NoError(err)
	}
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package engine

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata/token.wasm
	tokenProgramBytes []byte
)

// go test -v -benchmem -run=^$ -bench ^BenchmarkCompileModule$ github.com/ava-labs/hypersdk/x/programs/engine -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkCompileModule(b *testing.B) {
	require := require.New(b)
	eng := New(NewConfig())
	b.Run("benchmark_compile_wasm_no_cache", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := eng.CompileModule(tokenProgramBytes)
			require.NoError(err)
		}
	})

	cfg := NewConfig()
	err := cfg.CacheConfigLoadDefault()
	require.NoError(err)
	eng = New(cfg)
	require.NoError(err)
	b.Run("benchmark_compile_wasm_with_cache", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := eng.CompileModule(tokenProgramBytes)
			require.NoError(err)
		}
	})
}

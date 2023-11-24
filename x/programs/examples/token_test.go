// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	_ "embed"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/x/programs/examples/imports/pstate"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

var (
	//go:embed testdata/token.wasm
	tokenProgramBytes []byte

	log = logging.NewLogger(
		"",
		logging.NewWrappedCore(
			logging.Info,
			os.Stderr,
			logging.Plain.ConsoleEncoder(),
		))
)

// go test -v -timeout 30s -run ^TestTokenProgram$ github.com/ava-labs/hypersdk/x/programs/examples -memprofile benchvset.mem -cpuprofile benchvset.cpu
func TestTokenProgram(t *testing.T) {
	require := require.New(t)
	maxUnits := uint64(40000)
	cfg, err := runtime.NewConfigBuilder().Build()
	require.NoError(err)
	program, err := newTokenProgram(maxUnits, cfg, tokenProgramBytes)
	require.NoError(err)
	err = program.Run(context.Background())
	require.NoError(err)
}

// go test -v -benchmem -run=^$ -bench ^BenchmarkTokenProgram$ github.com/ava-labs/hypersdk/x/programs/examples -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkTokenProgram(b *testing.B) {
	require := require.New(b)
	maxUnits := uint64(40000)

	cfg, err := runtime.NewConfigBuilder().
		WithCompileStrategy(runtime.CompileWasm).
		WithDefaultCache(true).
		Build()
	require.NoError(err)

	b.Run("benchmark_token_program_compile_and_cache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			program, err := newTokenProgram(maxUnits, cfg, tokenProgramBytes)
			require.NoError(err)
			b.StartTimer()
			err = program.Run(context.Background())
			require.NoError(err)
		}
	})

	b.Run("benchmark_token_program_compile_and_cache_short", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			program, err := newTokenProgram(maxUnits, cfg, tokenProgramBytes)
			require.NoError(err)
			b.StartTimer()
			err = program.RunShort(context.Background())
			require.NoError(err)
		}
	})

	cfg, err = runtime.NewConfigBuilder().
		WithCompileStrategy(runtime.PrecompiledWasm).
		WithDefaultCache(false).
		Build()
	require.NoError(err)
	preCompiledTokenProgramBytes, err := runtime.PreCompileWasmBytes(tokenProgramBytes, cfg)
	require.NoError(err)

	b.ResetTimer()
	b.Run("benchmark_token_program_precompile", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			program, err := newTokenProgram(maxUnits, cfg, preCompiledTokenProgramBytes)
			require.NoError(err)
			b.StartTimer()
			err = program.Run(context.Background())
			require.NoError(err)
		}
	})

	b.Run("benchmark_token_program_precompile_short", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			program, err := newTokenProgram(maxUnits, cfg, preCompiledTokenProgramBytes)
			require.NoError(err)
			b.StartTimer()
			err = program.RunShort(context.Background())
			require.NoError(err)
		}
	})
}

func newTokenProgram(maxUnits uint64, cfg *runtime.Config, programBytes []byte) (*Token, error) {
	db := newTestDB()

	// define imports
	supported := runtime.NewSupportedImports()
	supported.Register("state", func() runtime.Import {
		return pstate.New(log, db)
	})
	return NewToken(log, programBytes, db, cfg, supported.Imports(), maxUnits), nil
}

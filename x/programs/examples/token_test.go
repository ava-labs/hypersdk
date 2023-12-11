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

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/pstate"
	"github.com/ava-labs/hypersdk/x/programs/host"
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
	cfg := runtime.NewConfig()
	eng := engine.New(engine.NewConfig())
	program, err := newTokenProgram(eng, maxUnits, cfg, tokenProgramBytes)
	require.NoError(err)
	err = program.Run(context.Background())
	require.NoError(err)
}

// go test -v -benchmem -run=^$ -bench ^BenchmarkTokenProgram$ github.com/ava-labs/hypersdk/x/programs/examples -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkTokenProgram(b *testing.B) {
	require := require.New(b)
	maxUnits := uint64(40000)
	cfg := runtime.NewConfig().
		WithCompileStrategy(engine.CompileWasm)
	ecfg, err := engine.NewConfigBuilder().
		WithDefaultCache(true).
		Build()
	require.NoError(err)
	eng := engine.New(ecfg)
	b.Run("benchmark_token_program_compile_and_cache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			program, err := newTokenProgram(eng, maxUnits, cfg, tokenProgramBytes)
			require.NoError(err)
			b.StartTimer()
			err = program.Run(context.Background())
			require.NoError(err)
		}
	})

	b.Run("benchmark_token_program_compile_and_cache_short", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			program, err := newTokenProgram(eng, maxUnits, cfg, tokenProgramBytes)
			require.NoError(err)
			b.StartTimer()
			err = program.RunShort(context.Background())
			require.NoError(err)
		}
	})

	rcfg := runtime.NewConfig().
		WithCompileStrategy(engine.PrecompiledWasm)
	ecfg, err = engine.NewConfigBuilder().
		WithDefaultCache(false).
		Build()
	require.NoError(err)
	preCompiledTokenProgramBytes, err := engine.PreCompileWasmBytes(tokenProgramBytes, ecfg, rcfg.LimitMaxMemory)
	require.NoError(err)

	b.ResetTimer()
	b.Run("benchmark_token_program_precompile", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			program, err := newTokenProgram(eng, maxUnits, rcfg, preCompiledTokenProgramBytes)
			require.NoError(err)
			b.StartTimer()
			err = program.Run(context.Background())
			require.NoError(err)
		}
	})

	b.Run("benchmark_token_program_precompile_short", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			program, err := newTokenProgram(eng, maxUnits, cfg, preCompiledTokenProgramBytes)
			require.NoError(err)
			b.StartTimer()
			err = program.RunShort(context.Background())
			require.NoError(err)
		}
	})
}

func newTokenProgram(engine *engine.Engine, maxUnits uint64, cfg *runtime.Config, programBytes []byte) (*Token, error) {
	db := newTestDB()

	// define imports
	imports := host.NewImportsBuilder()
	imports.Register("state", func() host.Import {
		return pstate.New(log, db)
	})
	return NewToken(log, engine, programBytes, db, cfg, imports.Build(), maxUnits), nil
}

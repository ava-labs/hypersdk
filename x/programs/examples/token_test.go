// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"os"
	"testing"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/pstate"
	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/ava-labs/hypersdk/x/programs/tests"
)

// go test -v -timeout 30s -run ^TestTokenProgram$ github.com/ava-labs/hypersdk/x/programs/examples -memprofile benchvset.mem -cpuprofile benchvset.cpu
func TestTokenProgram(t *testing.T) {
	wasmBytes := tests.ReadFixture(t, "../tests/fixture/token.wasm")
	require := require.New(t)
	maxUnits := uint64(80000)
	eng := engine.New(engine.NewConfig())
	program := newTokenProgram(maxUnits, eng, runtime.NewConfig(), wasmBytes)
	require.NoError(program.Run(context.Background()))
}

// go test -v -benchmem -run=^$ -bench ^BenchmarkTokenProgram$ github.com/ava-labs/hypersdk/x/programs/examples -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkTokenProgram(b *testing.B) {
	wasmBytes := tests.ReadFixture(b, "../tests/fixture/token.wasm")
	maxUnits := uint64(80000)

	cfg := runtime.NewConfig().
		SetCompileStrategy(engine.CompileWasm)

	ecfg, err := engine.NewConfigBuilder().
		WithDefaultCache(true).
		Build()
	require.NoError(b, err)
	eng := engine.New(ecfg)

	b.Run("benchmark_token_program_compile_and_cache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			program := newTokenProgram(maxUnits, eng, cfg, wasmBytes)
			b.StartTimer()
			require.NoError(b, program.Run(context.Background()))
		}
	})

	b.Run("benchmark_token_program_compile_and_cache_short", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			program := newTokenProgram(maxUnits, eng, cfg, wasmBytes)
			b.StartTimer()
			require.NoError(b, program.RunShort(context.Background()))
		}
	})

	cfg = runtime.NewConfig().
		SetCompileStrategy(engine.PrecompiledWasm)
	ecfg, err = engine.NewConfigBuilder().
		WithDefaultCache(true).
		Build()
	eng = engine.New(ecfg)
	require.NoError(b, err)
	preCompiledTokenProgramBytes, err := engine.PreCompileWasmBytes(eng, wasmBytes, cfg.LimitMaxMemory)
	require.NoError(b, err)

	b.ResetTimer()
	b.Run("benchmark_token_program_precompile", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			program := newTokenProgram(maxUnits, eng, cfg, preCompiledTokenProgramBytes)
			b.StartTimer()
			require.NoError(b, program.Run(context.Background()))
		}
	})

	b.Run("benchmark_token_program_precompile_short", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			program := newTokenProgram(maxUnits, eng, cfg, preCompiledTokenProgramBytes)
			b.StartTimer()
			require.NoError(b, program.RunShort(context.Background()))
		}
	})
}

func newTokenProgram(maxUnits uint64, engine *engine.Engine, cfg *runtime.Config, programBytes []byte) *Token {
	db := newTestDB()

	log := logging.NewLogger(
		"",
		logging.NewWrappedCore(
			logging.Info,
			os.Stderr,
			logging.Plain.ConsoleEncoder(),
		))

	// define imports
	importsBuilder := host.NewImportsBuilder()
	importsBuilder.Register("state", func() host.Import {
		return pstate.New(log, db)
	})

	return NewToken(log, engine, programBytes, db, cfg, importsBuilder.Build(), maxUnits)
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"os"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/pstate"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/ava-labs/hypersdk/x/programs/tests"
)

// go test -v -timeout 30s -run ^TestTokenProgram$ github.com/ava-labs/hypersdk/x/programs/examples -memprofile benchvset.mem -cpuprofile benchvset.cpu
func TestTokenProgram(t *testing.T) {
	t.Run("BurnUserTokens", func(t *testing.T) {
		wasmBytes := tests.ReadFixture(t, "../tests/fixture/token.wasm")
		require := require.New(t)
		maxUnits := uint64(80000)
		eng := engine.New(engine.NewConfig())
		program := newTokenProgram(maxUnits, eng, runtime.NewConfig(), wasmBytes)
		require.NoError(program.Run(context.Background()))

		rt := runtime.New(program.log, program.engine, program.imports, program.cfg)
		ctx := context.Background()
		err := rt.Initialize(ctx, program.programBytes, program.maxUnits)
		require.NoError(err)

		// simulate create program transaction
		programID := ids.GenerateTestID()
		err = storage.SetProgram(ctx, program.db, programID, program.programBytes)
		require.NoError(err)

		mem, err := rt.Memory()
		require.NoError(err)

		programIDPtr, err := argumentToSmartPtr(programID, mem)
		require.NoError(err)

		// initialize program
		_, err = rt.Call(ctx, "init", programIDPtr)
		require.NoError(err, "failed to initialize program")

		// generate alice keys
		alicePublicKey, err := newKey()
		require.NoError(err)

		// write alice's key to stack and get pointer
		alicePtr, err := argumentToSmartPtr(alicePublicKey, mem)
		require.NoError(err)

		// mint 100 tokens to alice
		mintAlice := int64(1000)
		mintAlicePtr, err := argumentToSmartPtr(mintAlice, mem)
		require.NoError(err)

		_, err = rt.Call(ctx, "mint_to", programIDPtr, alicePtr, mintAlicePtr)
		require.NoError(err)

		// check balance of alice
		result, err := rt.Call(ctx, "get_balance", programIDPtr, alicePtr)
		require.NoError(err)
		require.Equal(int64(1000), result[0])

		// burn alice tokens
		_, err = rt.Call(ctx, "burn_from", programIDPtr, alicePtr)
		require.NoError(err)

		// check balance of alice
		result, err = rt.Call(ctx, "get_balance", programIDPtr, alicePtr)
		require.NoError(err)
		require.Equal(int64(0), result[0])
	})

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

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
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/ava-labs/hypersdk/x/programs/utils"
)

var (
	//go:embed testdata/token.wasm
	tokenProgramBytes []byte

	log = logging.NewLogger(
		"",
		logging.NewWrappedCore(
			logging.Debug,
			os.Stderr,
			logging.Plain.ConsoleEncoder(),
		))
)

// go test -v -timeout 30s -run ^TestTokenProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestTokenProgram(t *testing.T) {
	require := require.New(t)
	db := utils.NewTestDB()
	maxUnits := uint64(50000)
	// define imports
	imports := make(runtime.Imports)
	imports["state"] = state.New(log, db)

	cfg, err := runtime.NewConfigBuilder(maxUnits).
		WithLimitMaxMemory(60 * 64 * units.KiB). // 17 pages
		Build()
	require.NoError(err)
	program := NewToken(log, tokenProgramBytes, db, cfg, imports)
	err = program.Run(context.Background())
	require.NoError(err)
}

// go test -v -benchmem -run=^$ -bench ^BenchmarkTokenProgram$ github.com/ava-labs/hypersdk/x/programs/examples -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkTokenProgram(b *testing.B) {
	require := require.New(b)
	maxUnits := uint64(40000)

	b.Run("benchmark_token_program_compile_and_cache", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			// configs can only be used once
			cfg, err := runtime.NewConfigBuilder(maxUnits).
				WithBulkMemory(true).
				WithLimitMaxMemory(17 * 64 * units.KiB). // 17 pages
				WithDefaultCache(true).
				Build()
			require.NoError(err)
			db := utils.NewTestDB()

			// define imports
			imports := make(runtime.Imports)
			imports["state"] = state.New(log, db)
			program := NewToken(log, tokenProgramBytes, db, cfg, imports)
			b.StartTimer()

			err = program.Run(context.Background())
			require.NoError(err)
		}
	})

	cfg, err := runtime.NewConfigBuilder(maxUnits).
		WithBulkMemory(true).
		WithLimitMaxMemory(17 * 64 * units.KiB). // 17 pages
		Build()
	require.NoError(err)
	preCompiledTokenProgramBytes, err := runtime.PreCompileWasmBytes(tokenProgramBytes, cfg)
	require.NoError(err)

	b.ResetTimer()
	b.Run("benchmark_token_program_precompile", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			cfg, err := runtime.NewConfigBuilder(maxUnits).
				WithBulkMemory(true).
				WithLimitMaxMemory(17 * 64 * units.KiB). // 17 pages
				WithCompileStrategy(runtime.PrecompiledWasm).
				Build()
			require.NoError(err)

			// setup db
			db := utils.NewTestDB()

			// define imports
			imports := make(runtime.Imports)
			imports["state"] = state.New(log, db)
			program := NewToken(log, preCompiledTokenProgramBytes, db, cfg, imports)
			b.StartTimer()

			err = program.Run(context.Background())
			require.NoError(err)
		}
	})
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	_ "embed"
	"os"
	"testing"

	// "github.com/bytecodealliance/wasmtime-go/v12"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/hashmap"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/ava-labs/hypersdk/x/programs/utils"
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

// go test -v -timeout 30s -run ^TestTokenWazeroProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestTokenProgram(t *testing.T) {
	require := require.New(t)
	maxUnits := uint64(40000)
	db := utils.NewTestDB()

	// define imports
	imports := make(runtime.Imports)
	imports["map"] = hashmap.New(log, db)

	cfg := runtime.NewConfigBuilder(maxUnits).
		WithBulkMemory(true).
		WithLimitMaxMemory(17 * 64 * 1024). // 17 pages
		Build()
	program := NewToken(log, tokenProgramBytes, cfg, imports)
	err := program.Run(context.Background())
	require.NoError(err)
}

// go test -v -benchmem -run=^$ -bench ^BenchmarkTokenProgram$ github.com/ava-labs/hypersdk/x/programs/examples -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkTokenWazeroProgram(b *testing.B) {
	require := require.New(b)
	maxUnits := uint64(40000)
	db := utils.NewTestDB()

	// define imports
	imports := make(runtime.Imports)
	imports["map"] = hashmap.New(log, db)

	b.Run("benchmark_token_program", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			cfg := runtime.NewConfigBuilder(maxUnits).
				WithBulkMemory(true).
				WithLimitMaxMemory(17 * 64 * 1024). // 17 pages
				WithDefaultCache(true).
				Build()

			program := NewToken(log, tokenProgramBytes, cfg, imports)
			b.StartTimer()
			err := program.Run(context.Background())
			require.NoError(err)
		}
	})
}

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
)

var (
	//go:embed testdata/token.wasm
	tokenProgramBytes []byte

	// example cost map
	costMap = map[string]uint64{
		"ConstI32 0x0": 1,
		"ConstI64 0x0": 2,
	}
	maxGas uint64 = 13000
	log           = logging.NewLogger(
		"",
		logging.NewWrappedCore(
			logging.Info,
			os.Stderr,
			logging.Plain.ConsoleEncoder(),
		))
)

// go test -v -timeout 30s -run ^TestTokenWazeroProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestTokenWazeroProgram(t *testing.T) {
	require := require.New(t)
	program := NewTokenWazero(log, tokenProgramBytes, maxGas, costMap)
	err := program.Run(context.Background())
	require.NoError(err)
}

// go test -v -timeout 30s -run ^TestTokenWazeroProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestTokenWasmtimeProgram(t *testing.T) {
	require := require.New(t)
	program := NewTokenWasmtime(log, tokenProgramBytes, maxGas, costMap)
	err := program.Run(context.Background())
	require.NoError(err)
}


// go test -v -benchmem -run=^$ -bench ^BenchmarkTokenProgram$ github.com/ava-labs/hypersdk/x/programs/examples -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkTokenWazeroProgram(b *testing.B) {
	require := require.New(b)
	program := NewTokenWazero(log, tokenProgramBytes, maxGas, costMap)
	b.ResetTimer()
	b.Run("benchmark_token_program_wazero", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := program.Run(context.Background())
			require.NoError(err)
		}
	})
}


func BenchmarkTokenWasmtimeProgram(b *testing.B) {
	require := require.New(b)
	program := NewTokenWasmtime(log, tokenProgramBytes, maxGas, costMap)
	b.ResetTimer()
	b.Run("benchmark_token_program_wasmtime", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := program.Run(context.Background())
			require.NoError(err)
		}
	})
}

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

	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

var (
	//go:embed testdata/token_program.wasm
	tokenProgramBytes []byte

	// example cost map
	costMap = map[string]uint64{
		"ConstI32 0x0": 1,
		"ConstI64 0x0": 2,
	}
	maxGas uint64 = 3000
)

// go test -v -timeout 30s -run ^TestTokenProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestTokenProgram(t *testing.T) {
	require := require.New(t)
	log := newLoggerWithLogLevel(logging.Debug)
	meter := runtime.NewMeter(log, maxGas, costMap)
	program := NewToken(log, tokenProgramBytes)
	err := program.Run(context.Background(), meter)
	require.NoError(err)
}

// go test -v -benchmem -run=^$ -bench ^BenchmarkTokenProgram$ github.com/ava-labs/hypersdk/x/programs/examples -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkTokenProgram(b *testing.B) {
	require := require.New(b)
	log := newLoggerWithLogLevel(logging.Info)
	b.Run("benchmark_token_program", func(b *testing.B) {
		program := NewToken(log, tokenProgramBytes)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			meter := runtime.NewMeter(log, maxGas, costMap)
			err := program.Run(context.Background(), meter)
			require.NoError(err)
		}
	})
	b.Run("benchmark_token_program_no_metering", func(b *testing.B) {
		program := NewToken(log, tokenProgramBytes)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var meter runtime.Meter // disabled
			err := program.Run(context.Background(), meter)
			require.NoError(err)
		}
	})
}

func newLoggerWithLogLevel(level logging.Level) logging.Logger {
	return logging.NewLogger(
		"",
		logging.NewWrappedCore(
			level,
			os.Stderr,
			logging.Plain.ConsoleEncoder(),
		))
}

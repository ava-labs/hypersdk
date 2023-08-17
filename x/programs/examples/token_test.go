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
	//go:embed testdata/token_contract.wasm
	tokenProgramBytes []byte

	// example cost map
	costMap = map[string]uint64{
		"ConstI32 0x0": 1,
		"ConstI64 0x0": 2,
	}
	maxGas uint64 = 3000
	log           = logging.NewLogger(
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
	program := NewToken(log, tokenProgramBytes, maxGas, costMap)
	err := program.Run(context.Background())
	require.NoError(err)
}

// go test -v -benchmem -run=^$ -bench ^BenchmarkTokenProgram$ github.com/ava-labs/hypersdk/x/programs/examples -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkTokenProgram(b *testing.B) {
	require := require.New(b)
	program := NewToken(log, tokenProgramBytes, maxGas, costMap)
	b.ResetTimer()
	b.Run("benchmark_token_program", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := program.Run(context.Background())
			require.NoError(err)
		}
	})
}

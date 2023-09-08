// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed testdata/token.wasm
var tokenProgramBytes []byte

// go test -v -timeout 30s -run ^TestTokenProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestTokenProgram(t *testing.T) {
	require := require.New(t)
	program := NewToken(log, tokenProgramBytes, DefaultMaxFee, CostMap)
	err := program.Run(context.Background())
	require.NoError(err)
}

// go test -v -benchmem -run=^$ -bench ^BenchmarkTokenProgram$ github.com/ava-labs/hypersdk/x/programs/examples -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkTokenProgram(b *testing.B) {
	require := require.New(b)
	program := NewToken(log, tokenProgramBytes, DefaultMaxFee, CostMap)
	b.ResetTimer()
	b.Run("benchmark_token_program", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := program.Run(context.Background())
			require.NoError(err)
		}
	})
}

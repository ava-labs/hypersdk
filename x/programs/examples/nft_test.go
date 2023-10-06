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
	nftProgramBytes []byte

	// example cost map
	nftCostMap = map[string]uint64{
		"ConstI32 0x0": 1,
		"ConstI64 0x0": 2,
	}
	nftMaxGas uint64 = 13000
	nftLog           = logging.NewLogger(
		"",
		logging.NewWrappedCore(
			logging.Debug,
			os.Stderr,
			logging.Plain.ConsoleEncoder(),
		))

	metadata = Metadata{
		Name:   "My NFT",
		Symbol: "MNFT",
		URI:    "ipfs://my-nft.jpg",
	}
)

// go test -v -timeout 30s -run ^TestNFTProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestNFTProgram(t *testing.T) {
	require := require.New(t)

	program := NewNFT(nftLog, nftProgramBytes, nftMaxGas, nftCostMap, metadata)
	err := program.Run(context.Background())
	require.NoError(err)
}

// go test -v -benchmark -run=^$ -bench ^BenchmarkNFTProgram$ github.com/ava-labs/hypersdk/x/programs/examples -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkNFTProgram(b *testing.B) {
	require := require.New(b)
	program := NewNFT(nftLog, nftProgramBytes, nftMaxGas, nftCostMap, metadata)
	b.ResetTimer()
	b.Run("benchmark_nft_program", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := program.Run(context.Background())
			require.NoError(err)
		}
	})
}

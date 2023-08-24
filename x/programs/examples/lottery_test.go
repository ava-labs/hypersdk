// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	//go:embed testdata/lottery.wasm
	lotteryProgramBytes []byte
)

// go test -v -timeout 30s -run ^TestLotteryProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestLotteryProgram(t *testing.T) {
	require := require.New(t)
	program := NewLottery(log, lotteryProgramBytes, tokenProgramBytes, maxGas, costMap)
	err := program.Run(context.Background())
	require.NoError(err)
}

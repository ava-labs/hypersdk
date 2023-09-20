// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	_ "embed"
	"testing"

	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/hashmap"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/program"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/ava-labs/hypersdk/x/programs/utils"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/lottery.wasm
var lotteryProgramBytes []byte

// go test -v -timeout 30s -run ^TestLotteryProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestLotteryProgram(t *testing.T) {
	require := require.New(t)
	maxUnits := uint64(40000)
	db := utils.NewTestDB()

	// define imports
	imports := make(runtime.Imports)
	imports["map"] = hashmap.New(log, db)
	imports["program"] = program.New(log, db)


	lcfg, err := runtime.NewConfigBuilder(maxUnits).
		WithBulkMemory(true).
		WithLimitMaxMemory(17 * 64 * units.KiB). // 17 pages
		Build()
	require.NoError(err)

	tcfg, err := runtime.NewConfigBuilder(maxUnits).
		WithBulkMemory(true).
		WithLimitMaxMemory(17 * 64 * units.KiB). // 17 pages
		Build()
	require.NoError(err)
	program := NewLottery(log, lotteryProgramBytes, lcfg, tcfg, imports)
	err = program.Run(context.Background())
	require.NoError(err)
}

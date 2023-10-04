// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	_ "embed"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/program"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/pstate"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/ava-labs/hypersdk/x/programs/utils"
)

//go:embed testdata/counter.wasm
var counterProgramBytes []byte

// go test -v -timeout 30s -run ^TestCounterProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestCounterProgram(t *testing.T) {
	require := require.New(t)
	db := utils.NewTestDB()
	maxUnits := uint64(5000000)

	// define supported imports
	imports := runtime.NewSupportedImports()
	imports["state"] = func() runtime.Import {
		return pstate.New(log, db)
	}
	imports["program"] = func() runtime.Import {
		return program.New(log, db)
	}

	cfg, err := runtime.NewConfigBuilder(maxUnits).
		WithLimitMaxMemory(18 * 64 * units.KiB). // 17 pages
		WithBulkMemory(true).
		Build()
	require.NoError(err)

	cfg2, err := runtime.NewConfigBuilder(maxUnits).
		WithLimitMaxMemory(18 * 64 * units.KiB). // 17 pages
		WithBulkMemory(true).
		Build()
	require.NoError(err)

	program := NewCounter(log, counterProgramBytes, db, cfg, cfg2, imports)
	err = program.Run(context.Background())
	require.NoError(err)
}

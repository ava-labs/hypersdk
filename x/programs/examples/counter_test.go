// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/program"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/pstate"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/ava-labs/hypersdk/x/programs/tests"
)

// go test -v -timeout 30s -run ^TestCounterProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestCounterProgram(t *testing.T) {
	require := require.New(t)
	db := newTestDB()
	maxUnits := uint64(80000)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := runtime.NewConfig()
	log := logging.NewLogger(
		"",
		logging.NewWrappedCore(
			logging.Info,
			os.Stderr,
			logging.Plain.ConsoleEncoder(),
		))

	eng := engine.New(engine.NewConfig())
	// define supported imports
	supported := runtime.NewSupportedImports()
	supported.Register("state", func() runtime.Import {
		return pstate.New(log, db)
	})
	supported.Register("program", func() runtime.Import {
		return program.New(log, eng, db, cfg)
	})

	wasmBytes := tests.ReadFixture(t, "../tests/fixture/counter.wasm")
	rt := runtime.New(log, eng, supported.Imports(), cfg)
	err := rt.Initialize(ctx, wasmBytes, maxUnits)
	require.NoError(err)

	balance, err := rt.Meter().GetBalance()
	require.NoError(err)
	require.Equal(maxUnits, balance)

	// simulate create program transaction
	programID := ids.GenerateTestID()
	err = storage.SetProgram(ctx, db, programID, wasmBytes)
	require.NoError(err)

	programIDPtr, err := argumentToSmartPtr(programID, rt.Memory())
	require.NoError(err)

	// generate alice keys
	_, aliceKey, err := newKey()
	require.NoError(err)

	// write alice's key to stack and get pointer
	alicePtr, err := argumentToSmartPtr(aliceKey, rt.Memory())
	require.NoError(err)

	// create counter for alice on program 1
	result, err := rt.Call(ctx, "initialize_address", programIDPtr, alicePtr)
	require.NoError(err)
	require.Equal(int64(1), result[0])

	// validate counter at 0
	result, err = rt.Call(ctx, "get_value", programIDPtr, alicePtr)
	require.NoError(err)
	require.Equal(int64(0), result[0])

	// initialize second runtime to create second counter program with an empty
	// meter.
	rt2 := runtime.New(log, eng, supported.Imports(), cfg)
	err = rt2.Initialize(ctx, wasmBytes, engine.NoUnits)

	require.NoError(err)

	// define max units to transfer to second runtime
	unitsTransfer := uint64(10000)

	// transfer the units from the original runtime to the new runtime before
	// any calls are made.
	_, err = rt.Meter().TransferUnitsTo(rt2.Meter(), unitsTransfer)
	require.NoError(err)

	// simulate creating second program transaction
	program2ID := ids.GenerateTestID()
	err = storage.SetProgram(ctx, db, program2ID, wasmBytes)
	require.NoError(err)

	programID2Ptr, err := argumentToSmartPtr(program2ID, rt2.Memory())
	require.NoError(err)

	// write alice's key to stack and get pointer
	alicePtr2, err := argumentToSmartPtr(aliceKey, rt2.Memory())
	require.NoError(err)

	// initialize counter for alice on runtime 2
	result, err = rt2.Call(ctx, "initialize_address", programID2Ptr, alicePtr2)
	require.NoError(err)
	require.Equal(int64(1), result[0])

	// increment alice's counter on program 2 by 10
	incAmount := int64(10)
	incAmountPtr, err := argumentToSmartPtr(incAmount, rt2.Memory())
	require.NoError(err)
	result, err = rt2.Call(ctx, "inc", programID2Ptr, alicePtr2, incAmountPtr)
	require.NoError(err)
	require.Equal(int64(1), result[0])

	result, err = rt2.Call(ctx, "get_value", programID2Ptr, alicePtr2)
	require.NoError(err)
	require.Equal(incAmount, result[0])

	balance, err = rt2.Meter().GetBalance()
	require.NoError(err)

	// transfer balance back to original runtime
	_, err = rt2.Meter().TransferUnitsTo(rt.Meter(), balance)
	require.NoError(err)

	// increment alice's counter on program 1
	onePtr, err := argumentToSmartPtr(int64(1), rt.Memory())
	require.NoError(err)
	result, err = rt.Call(ctx, "inc", programIDPtr, alicePtr, onePtr)
	require.NoError(err)
	require.Equal(int64(1), result[0])

	result, err = rt.Call(ctx, "get_value", programIDPtr, alicePtr)
	require.NoError(err)

	log.Debug("count program 1",
		zap.Int64("alice", result[0]),
	)

	// write program id 2 to stack of program 1
	programID2Ptr, err = argumentToSmartPtr(program2ID, rt.Memory())
	require.NoError(err)

	caller := programIDPtr
	target := programID2Ptr
	maxUnitsProgramToProgram := int64(10000)
	maxUnitsProgramToProgramPtr, err := argumentToSmartPtr(maxUnitsProgramToProgram, rt.Memory())
	require.NoError(err)

	// increment alice's counter on program 2
	fivePtr, err := argumentToSmartPtr(int64(5), rt.Memory())
	require.NoError(err)
	result, err = rt.Call(ctx, "inc_external", caller, target, maxUnitsProgramToProgramPtr, alicePtr, fivePtr)
	require.NoError(err)
	require.Equal(int64(1), result[0])

	// expect alice's counter on program 2 to be 15
	result, err = rt.Call(ctx, "get_value_external", caller, target, maxUnitsProgramToProgramPtr, alicePtr)
	require.NoError(err)
	require.Equal(int64(15), result[0])
	balance, err = rt.Meter().GetBalance()
	require.NoError(err)
	require.Greater(balance, uint64(0))
}

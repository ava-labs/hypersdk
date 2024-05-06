// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	"os"
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/pstate"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/program"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	"github.com/ava-labs/hypersdk/x/programs/tests"

	iprogram "github.com/ava-labs/hypersdk/x/programs/examples/imports/program"
)

// go test -v -timeout 30s -run ^TestCounterProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestCounterProgram(t *testing.T) {
	require := require.New(t)
	db := newTestDB()
	maxUnits := uint64(10000000)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	programID := ids.GenerateTestID()
	callContext := program.Context{ProgramID: programID}
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
	importsBuilder := host.NewImportsBuilder()
	importsBuilder.Register("state", func() host.Import {
		return pstate.New(log, db)
	})

	importsBuilder.Register("program", func() host.Import {
		return iprogram.New(log, eng, db, cfg, &callContext)
	})
	imports := importsBuilder.Build()

	wasmBytes := tests.ReadFixture(t, "../tests/fixture/counter.wasm")
	rt := runtime.New(log, eng, imports, cfg)

	err := rt.Initialize(ctx, callContext, wasmBytes, maxUnits)
	require.NoError(err)

	balance, err := rt.Meter().GetBalance()
	require.NoError(err)
	require.Equal(maxUnits, balance)

	// simulate create program transaction

	err = storage.SetProgram(ctx, db, programID, wasmBytes)
	require.NoError(err)

	mem, err := rt.Memory()
	require.NoError(err)

	// generate alice keys
	alicePublicKey, err := newKey()
	require.NoError(err)

	// write alice's key to stack and get pointer
	alicePtr, err := writeToMem(alicePublicKey, mem)
	require.NoError(err)

	// create counter for alice on program 1
	result, err := rt.Call(ctx, "initialize_address", callContext, alicePtr)
	require.NoError(err)
	require.Equal(int64(1), result[0])

	alicePtr, err = writeToMem(alicePublicKey, mem)
	require.NoError(err)

	// validate counter at 0
	result, err = rt.Call(ctx, "get_value", callContext, alicePtr)
	require.NoError(err)
	require.Equal(int64(0), result[0])

	// initialize second runtime to create second counter program with an empty
	// meter.
	rt2 := runtime.New(log, eng, imports, cfg)
	err = rt2.Initialize(ctx, callContext, wasmBytes, engine.NoUnits)

	require.NoError(err)

	// define max units to transfer to second runtime
	unitsTransfer := uint64(2000000)

	// transfer the units from the original runtime to the new runtime before
	// any calls are made.
	_, err = rt.Meter().TransferUnitsTo(rt2.Meter(), unitsTransfer)
	require.NoError(err)

	// simulate creating second program transaction
	programID2 := ids.GenerateTestID()
	err = storage.SetProgram(ctx, db, programID2, wasmBytes)
	require.NoError(err)

	mem2, err := rt2.Memory()
	require.NoError(err)

	// write alice's key to stack and get pointer
	alicePtr2, err := writeToMem(alicePublicKey, mem2)
	require.NoError(err)

	callContext1 := program.Context{ProgramID: programID}
	callContext2 := program.Context{ProgramID: programID2}

	// initialize counter for alice on runtime 2
	result, err = rt2.Call(ctx, "initialize_address", callContext2, alicePtr2)
	require.NoError(err)
	require.Equal(int64(1), result[0])

	// increment alice's counter on program 2 by 10
	incAmount := int64(10)
	incAmountPtr, err := writeToMem(incAmount, mem2)
	require.NoError(err)

	alicePtr2, err = writeToMem(alicePublicKey, mem2)

	require.NoError(err)
	result, err = rt2.Call(ctx, "inc", callContext2, alicePtr2, incAmountPtr)
	require.NoError(err)
	require.Equal(int64(1), result[0])

	alicePtr2, err = writeToMem(alicePublicKey, mem2)
	require.NoError(err)

	result, err = rt2.Call(ctx, "get_value", callContext2, alicePtr2)
	require.NoError(err)
	require.Equal(incAmount, result[0])

	balance, err = rt2.Meter().GetBalance()
	require.NoError(err)

	// transfer balance back to original runtime
	_, err = rt2.Meter().TransferUnitsTo(rt.Meter(), balance)
	require.NoError(err)

	// increment alice's counter on program 1
	onePtr, err := writeToMem(int64(1), mem)
	require.NoError(err)

	alicePtr, err = writeToMem(alicePublicKey, mem)
	require.NoError(err)

	result, err = rt.Call(ctx, "inc", callContext1, alicePtr, onePtr)
	require.NoError(err)
	require.Equal(int64(1), result[0])

	alicePtr, err = writeToMem(alicePublicKey, mem)
	require.NoError(err)

	result, err = rt.Call(ctx, "get_value", callContext1, alicePtr)
	require.NoError(err)

	log.Debug("count program 1",
		zap.Int64("alice", result[0]),
	)

	// write program id 2 to stack of program 1
	target, err := writeToMem(programID2, mem)
	require.NoError(err)

	maxUnitsProgramToProgram := int64(1000000)
	maxUnitsProgramToProgramPtr, err := writeToMem(maxUnitsProgramToProgram, mem)
	require.NoError(err)

	// increment alice's counter on program 2
	fivePtr, err := writeToMem(int64(5), mem)
	require.NoError(err)
	alicePtr, err = writeToMem(alicePublicKey, mem)
	require.NoError(err)
	result, err = rt.Call(ctx, "inc_external", callContext1, target, maxUnitsProgramToProgramPtr, alicePtr, fivePtr)
	require.NoError(err)
	require.Equal(int64(1), result[0])

	target, err = writeToMem(programID2, mem)
	require.NoError(err)
	alicePtr, err = writeToMem(alicePublicKey, mem)
	require.NoError(err)
	maxUnitsProgramToProgramPtr, err = writeToMem(maxUnitsProgramToProgram, mem)
	require.NoError(err)
	// expect alice's counter on program 2 to be 15
	result, err = rt.Call(ctx, "get_value_external", callContext1, target, maxUnitsProgramToProgramPtr, alicePtr)
	require.NoError(err)
	require.Equal(int64(15), result[0])
	balance, err = rt.Meter().GetBalance()
	require.NoError(err)
	require.Greater(balance, uint64(0))
}

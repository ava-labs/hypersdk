// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package examples

import (
	"context"
	_ "embed"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/program"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/pstate"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

//go:embed testdata/verify.wasm
var verifyProgramBytes []byte

// go test -v -timeout 30s -run ^TestVerifyProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestVerifyProgram(t *testing.T) {
	require := require.New(t)
	db := newTestDB()
	maxUnits := uint64(40000)
	// need with bulk memory to run this test(for io ops)
	cfg, err := runtime.NewConfigBuilder().WithDebugMode(true).Build()
	require.NoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// define supported imports
	supported := runtime.NewSupportedImports()
	supported.Register("state", func() runtime.Import {
		return pstate.New(log, db)
	})
	supported.Register("program", func() runtime.Import {
		return program.New(log, db, cfg)
	})
	supported.Register("wasiPreview1ModName", nil)



	rt := runtime.New(log, cfg, supported.Imports())
	err = rt.Initialize(ctx, verifyProgramBytes, maxUnits)
	require.NoError(err)

	require.Equal(maxUnits, rt.Meter().GetBalance())

	// simulate create program transaction
	programID := ids.GenerateTestID()
	err = storage.SetProgram(ctx, db, programID, verifyProgramBytes)
	require.NoError(err)

	programIDPtr, err := runtime.WriteBytes(rt.Memory(), programID[:])
	require.NoError(err)


	// call vertify
	_, err = rt.Call(ctx, "verify_ed_in_wasm", programIDPtr)
	fmt.Println(err)
	require.NoError(err)

	rt.Stop()

}

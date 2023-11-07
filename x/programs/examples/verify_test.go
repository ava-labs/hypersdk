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

var (
	secretKeyBytes = []byte{
		157, 97, 177, 157, 239, 253, 90, 96,
		186, 132, 074, 244, 146, 236, 044, 196,
		68, 073, 197, 105, 123, 050, 105, 025,
		112, 59, 172, 003, 28, 174, 127, 96,
	}

	wrongSecretKeyBytes = []byte{
		10, 97, 78, 157, 239, 253, 90, 96,
		186, 132, 074, 244, 146, 236, 044, 196,
		68, 073, 2, 105, 123, 050, 105, 025,
		112, 59, 172, 2, 28, 43, 127, 96,
	}

	messageBytes = []byte{
		84, 104, 105, 115, 32, 105, 115, 32, 
		97, 32, 116, 101, 115, 116, 32, 111,
		102, 32, 116, 104, 101, 32, 116, 115,
		117, 110, 97, 109, 105, 32, 97, 108,
	}
)


// go test -v -timeout 30s -run ^TestVerifyProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestVerifyProgram(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt, programIDPtr, err := SetupRuntime(ctx)
	require.NoError(err)
	meter := rt.Meter().GetBalance()
	
	signingBytes := grabSecretKeyBytes(4)
	// write bytes to memory
	secretKeyPtr, err := runtime.WriteBytes(rt.Memory(), signingBytes)
	require.NoError(err)
	messageBytesPtr, err := runtime.WriteBytes(rt.Memory(), messageBytes)
	require.NoError(err)
	// call vertify
	_, err = rt.Call(ctx, "verify_ed_in_wasm", programIDPtr, secretKeyPtr, messageBytesPtr)
	require.NoError(err)
	
	// check meter
	meter = meter - rt.Meter().GetBalance()
	fmt.Println("meter used: ", meter)
	


	
	rt.Stop()
}

func grabSecretKeyBytes(numCorrectKeys int) []byte {
	signingBytes := []byte{}
	for i := 0; i < numCorrectKeys; i++ {
		signingBytes = append(signingBytes, secretKeyBytes...)
	}
	signingBytes = append(signingBytes, wrongSecretKeyBytes...)
	return signingBytes
}

// go test -v -benchmem -run=^$ -bench ^BenchmarkVerifyProgram$ github.com/ava-labs/hypersdk/x/programs/examples -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkVerifyProgram(b *testing.B) {
	require := require.New(b)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	
	signingBytes := grabSecretKeyBytes(4)

	b.Run("benchmark_verify_inside_guest", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			rt, programIDPtr, err := SetupRuntime(ctx)
			require.NoError(err)
			// write bytes to memory(each time) wasm consumes memory
			secretKeyPtr, err := runtime.WriteBytes(rt.Memory(), signingBytes)
			require.NoError(err)
			messageBytesPtr, err := runtime.WriteBytes(rt.Memory(), messageBytes)
			require.NoError(err)
			b.StartTimer()
			_, err = rt.Call(ctx, "verify_ed_in_wasm", programIDPtr, secretKeyPtr, messageBytesPtr)
			require.NoError(err)
		}
	})
}


func SetupRuntime(ctx context.Context) (runtime.Runtime, uint64, error) {
	db := newTestDB()
	maxUnits := uint64(100000000)
	// need with bulk memory to run this test(for io ops)
	cfg, err := runtime.NewConfigBuilder().WithDebugMode(true).WithBulkMemory(true).Build()
	
	if err != nil {
		return nil, 0, err
	}	

	// define supported imports
	supported := runtime.NewSupportedImports()
	supported.Register("state", func() runtime.Import {
		return pstate.New(log, db)
	})
	supported.Register("program", func() runtime.Import {
		return program.New(log, db, cfg)
	})

	rt := runtime.New(log, cfg, supported.Imports())
	err = rt.Initialize(ctx, verifyProgramBytes, maxUnits)
	
	if err != nil {
		return nil, 0, err
	}	

	// simulate create program transaction
	programID := ids.GenerateTestID()
	err = storage.SetProgram(ctx, db, programID, verifyProgramBytes)
	if err != nil {
		return nil, 0, err
	}	

	programIDPtr, err := runtime.WriteBytes(rt.Memory(), programID[:])
	if err != nil {
		return nil, 0, err
	}	

	return rt, programIDPtr, nil
}
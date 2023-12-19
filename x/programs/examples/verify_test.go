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
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/crypto"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/program"
	"github.com/ava-labs/hypersdk/x/programs/examples/imports/pstate"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

//go:embed testdata/verify.wasm
var verifyProgramBytes []byte

var (
	signatureBytes   = [64]byte{138, 15, 65, 223, 37, 172, 140, 229, 29, 74, 112, 236, 253, 138, 180,
		244, 138, 132, 46, 10, 192, 213, 105, 102, 113, 101, 108, 225, 190, 53,
		186, 161, 105, 38, 179, 24, 6, 168, 146, 40, 42, 20, 242, 137, 52,
		74, 60, 50, 167, 2, 92, 98, 176, 17, 132, 30, 89, 110, 119, 239, 124, 40, 232, 14}

	messageBytes = [32]byte{109, 115, 103, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0}

	publicKey = [32]byte{
		115, 50, 124, 153, 59, 53, 196, 150, 168, 143, 151, 235,
		222, 128, 136, 161, 9, 40, 139, 85, 182, 153, 68, 135,
		62, 166, 45, 235, 251, 246, 69, 7,
	}

	invalidSignatureBytes = [64]byte{137, 15, 65, 223, 37, 172, 140, 229, 29, 74, 112, 236, 253, 138, 180,
		244, 138, 132, 46, 10, 192, 213, 105, 102, 113, 101, 108, 225, 190, 53,
		186, 161, 105, 38, 179, 24, 6, 168, 146, 40, 42, 20, 242, 137, 52,
		74, 60, 50, 167, 2, 92, 98, 176, 17, 132, 30, 89, 110, 119, 239, 124, 40, 232, 14}

)

// go test -v -timeout 30s -run ^TestVerifyProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestVerifyProgram(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt, programIDPtr, err := SetupRuntime(ctx)
	require.NoError(err)
	meter := rt.Meter().GetBalance()


	numValidMessages := 4
	numInvalidMessages := 1

	signedMessages := createSignedMessages(numValidMessages, numInvalidMessages)
	signedMessagesPtr, err := argumentToSmartPtr(signedMessages, rt.Memory())
	require.NoError(err)

	// call vertify with some invalid signatures
	result, err := rt.Call(ctx, "verify_ed_in_wasm", programIDPtr, signedMessagesPtr)
	require.NoError(err)
	require.Equal(int64(numValidMessages), result[0], "Verified an invalid # of signatures")

	// print meter for reference
	meter = meter - rt.Meter().GetBalance()
	fmt.Println("meter used: ", meter)

	rt.Stop()
}

// go test -v -timeout 30s -run ^TestVerifyHostFunctionProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestVerifyHostFunctionProgram(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt, programIDPtr, err := SetupRuntime(ctx)
	require.NoError(err)
	meter := rt.Meter().GetBalance()

	numValidMessages := 4
	numInvalidMessages := 1

	signedMessages := createSignedMessages(numValidMessages, numInvalidMessages)
	signedMessagesPtr, err := argumentToSmartPtr(signedMessages, rt.Memory())
	require.NoError(err)

	invalidSignedMessages := createSignedMessages(0, numInvalidMessages)
	invalidMessagesPtr, err := argumentToSmartPtr(invalidSignedMessages, rt.Memory())
	require.NoError(err)

	// call vertify with some invalid signatures
	result, err := rt.Call(ctx, "verify_ed_multiple_host_func", programIDPtr, signedMessagesPtr)
	require.NoError(err)
	require.Equal(int64(numValidMessages), result[0], "Verified an invalid # of signatures")

	// call vertify with all invalid signatures
	result, err = rt.Call(ctx, "verify_ed_multiple_host_func", programIDPtr, invalidMessagesPtr)
	require.NoError(err)
	require.Equal(int64(0), result[0], "Verified an invalid # of signatures")


	// print meter for reference
	meter = meter - rt.Meter().GetBalance()
	fmt.Println("meter used: ", meter)

	rt.Stop()
}


// go test -v -timeout 30s -run ^TestBatchVerifyHostFunctionProgram$ github.com/ava-labs/hypersdk/x/programs/examples
func TestBatchVerifyHostFunctionProgram(t *testing.T) {
	require := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rt, programIDPtr, err := SetupRuntime(ctx)
	require.NoError(err)
	meter := rt.Meter().GetBalance()

	numValidMessages := 4
	numInvalidMessages := 1

	signedMessages := createSignedMessages(numValidMessages, numInvalidMessages)
	signedMessagesPtr, err := argumentToSmartPtr(signedMessages, rt.Memory())
	require.NoError(err)

	invalidSignedMessages := createSignedMessages(0, numInvalidMessages)
	invalidMessagesPtr, err := argumentToSmartPtr(invalidSignedMessages, rt.Memory())
	require.NoError(err)

	// call vertify with some invalid signatures
	result, err := rt.Call(ctx, "verify_ed_batch_host_func", programIDPtr, signedMessagesPtr)
	require.NoError(err)
	require.Equal(int64(numValidMessages), result[0], "Verified an invalid # of signatures")

	// call vertify with all invalid signatures
	result, err = rt.Call(ctx, "verify_ed_batch_host_func", programIDPtr, invalidMessagesPtr)
	require.NoError(err)
	require.Equal(int64(0), result[0], "Verified an invalid # of signatures")


	// print meter for reference
	meter = meter - rt.Meter().GetBalance()
	fmt.Println("meter used: ", meter)

	rt.Stop()
}

// Helper function to create signed messages
func createSignedMessages(numValidMessages int, numInvalidMessages int) []crypto.SignedMessage {
	signedMessages := []crypto.SignedMessage{}
	for i := 0; i < numValidMessages; i++ {
		signedMessages = append(signedMessages, crypto.SignedMessage{
			Message:   messageBytes,
			Signature: signatureBytes,
			PublicKey: publicKey,
		})
	}
	for i := 0; i < numInvalidMessages; i++ {
		signedMessages = append(signedMessages, crypto.SignedMessage{
			Message:   messageBytes,
			Signature: invalidSignatureBytes,
			PublicKey: publicKey,
		})
	}
	return signedMessages
}

// go test -v -benchmem -run=^$ -bench ^BenchmarkVerifyProgram$ github.com/ava-labs/hypersdk/x/programs/examples -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkVerifyProgram(b *testing.B) {
	// require := require.New(b)
	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	// publicKeysBytes := grabPubKeyBytes(4)

	// b.Run("benchmark_verify_inside_guest", func(b *testing.B) {
	// 	for i := 0; i < b.N; i++ {
	// 		b.StopTimer()
	// 		rt, programIDPtr, err := SetupRuntime(ctx)
	// 		require.NoError(err)
	// 		// write bytes to memory(each time) wasm consumes memory
	// 		secretKeyPtr, err := runtime.WriteBytes(rt.Memory(), publicKeysBytes)
	// 		require.NoError(err)
	// 		messageBytesPtr, err := runtime.WriteBytes(rt.Memory(), messageBytes)
	// 		require.NoError(err)
	// 		b.StartTimer()
	// 		_, err = rt.Call(ctx, "verify_ed_in_wasm", programIDPtr, secretKeyPtr, messageBytesPtr)
	// 		require.NoError(err)
	// 	}
	// })
}

func SetupRuntime(ctx context.Context) (runtime.Runtime, runtime.SmartPtr, error) {
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
	supported.Register("crypto", func() runtime.Import {
		return crypto.New(log, db)
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

	programIDPtr, err := argumentToSmartPtr(programID, rt.Memory())
	if err != nil {
		return nil, 0, err
	}

	return rt, programIDPtr, nil
}
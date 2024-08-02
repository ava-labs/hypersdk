// include <stdint.h> required to import uint8_t
package main

/*
#cgo CFLAGS: -I../common
#include "types.h"
*/
import "C"

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"unsafe"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	simState "github.com/ava-labs/hypersdk/x/programs/simulator/state"
)

var (
	ErrInvalidParam     = errors.New("invalid parameter")
	ErrRuntimeExecution = errors.New("invalid parameter")
	SimLogger           = logging.NewLogger("Simulator")
	SimContext          = context.TODO()
)

//export CallProgram
func CallProgram(db *C.Mutable, ctx *C.SimulatorCallContext) C.CallProgramResponse {
	if db == nil || ctx == nil {
		return newCallProgramResponse(nil, 0, ErrInvalidParam)
	}

	// build the db
	state := simState.NewSimulatorState(unsafe.Pointer(db))
	// build the call info
	callInfo := createRuntimeCallInfo(state, ctx)

	rt := runtime.NewRuntime(runtime.NewConfig(), SimLogger)
	result, err := rt.CallProgram(SimContext, callInfo)

	if err != nil {
		return newCallProgramResponse(nil, 0, fmt.Errorf("error during runtime execution: %w", ErrRuntimeExecution))
	}

	fuel := callInfo.RemainingFuel()
	return newCallProgramResponse(result, fuel, nil)
}

func createRuntimeCallInfo(db state.Mutable, ctx *C.SimulatorCallContext) *runtime.CallInfo {
	paramBytes := C.GoBytes(unsafe.Pointer(ctx.params.data), C.int(ctx.params.length))
	methodName := C.GoString(ctx.method)
	return &runtime.CallInfo{
		State:        simState.NewProgramStateManager(db),
		Actor:        codec.Address(C.GoBytes(unsafe.Pointer(&ctx.actor_address), 33)),
		FunctionName: methodName,
		Program:      codec.Address(C.GoBytes(unsafe.Pointer(&ctx.program_address), 33)),
		Params:       paramBytes,
		Fuel:         uint64(ctx.max_gas),
		Height:       uint64(ctx.height),
		Timestamp:    uint64(ctx.timestamp),
	}
}

//export CreateProgram
func CreateProgram(db *C.Mutable, path *C.char) C.CreateProgramResponse {
	state := simState.NewSimulatorState(unsafe.Pointer(db))
	programManager := simState.NewProgramStateManager(state)

	programPath := C.GoString(path)
	programBytes, err := os.ReadFile(programPath)
	if err != nil {
		return C.CreateProgramResponse{
			error: C.CString(err.Error()),
		}
	}

	programID, err := generateRandomID()
	if err != nil {
		return C.CreateProgramResponse{
			error: C.CString(err.Error()),
		}
	}

	err = programManager.SetProgram(context.TODO(), programID, programBytes)
	if err != nil {
		errmsg := fmt.Sprintf("program creation failed: %s", err.Error())
		return C.CreateProgramResponse{
			error: C.CString(errmsg),
		}

	}

	account, err := programManager.DeployProgram(context.TODO(), programID, []byte{})
	if err != nil {
		errmsg := fmt.Sprintf("program deployment failed: %s", err.Error())
		return C.CreateProgramResponse{
			error: C.CString(errmsg),
		}
	}
	return C.CreateProgramResponse{
		error: nil,
		program_id: C.ID{
			id: *(*[32]C.uchar)(C.CBytes(programID[:])),
		},
		program_address: C.Address{
			address: *(*[33]C.uchar)(C.CBytes(account[:])),
		},
	}
}

// generateRandomID creates a unique ID.
// Note: ids.GenerateID() is not used because the IDs are not unique and will
// collide.
func generateRandomID() (ids.ID, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return ids.Empty, err
	}
	id, err := ids.ToID(key)
	if err != nil {
		return ids.Empty, err
	}

	return id, nil
}

func newCallProgramResponse(result []byte, fuel uint64, err error) C.CallProgramResponse {
	var errPtr *C.char
	if err == nil {
		errPtr = nil
	} else {
		errPtr = C.CString(err.Error())
	}
	// TODO: check what happens when result is nil
	return C.CallProgramResponse{
		error: errPtr,
		result: C.Bytes{
			data:   (*C.uint8_t)(C.CBytes(result)),
			length: C.uint(len(result)),
		},
		fuel: C.uint(fuel),
	}
}

func main() {}

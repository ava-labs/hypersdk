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

//export CallProgram
func CallProgram(db *C.Mutable) {
	// form db from params
	state := simState.NewSimulatorState(unsafe.Pointer(db))
	fmt.Println("Calling CallProgram")
	b, err := state.GetValue(context.TODO(), []byte{1, 2, 3})
   fmt.Println("b, err: ", b, err)
	state.Insert(context.TODO(), []byte{1, 2, 3}, []byte{6, 6, 9})
	b, err = state.GetValue(context.TODO(), []byte{1, 2, 3})
   fmt.Println("b, err: ", b, err)
	state.Remove(context.TODO(), []byte{1, 2, 3})
	state.GetValue(context.TODO(), []byte{1, 2, 3})

	fmt.Println("Triggering callback")
}


type ExecuteCtx struct {
   method string;
   paramBytes []byte;
   gas uint64;
}

//export Execute
func Execute(db *C.Mutable, ctx *C.SimulatorCallContext,  p *C.ExecutionRequest) C.Response {
   // TODO: error checking making sure the params are not nil
   
   // db
	state := simState.NewSimulatorState(unsafe.Pointer(db))
   // ctx
   
   testContext := createRuntimeContext(ctx);
   var paramBytes []byte
   // ExecutionRequest passed from the C code
   if p.params != nil {
      if p.paramLength == 0 {
         paramBytes = nil;
      } else {
         paramBytes = C.GoBytes(unsafe.Pointer(p.params), C.int(p.paramLength))
      }
   } else {
      paramBytes = nil
   }

	methodName := C.GoString(p.method)
   gas := p.maxGas

   executeCtx := ExecuteCtx{
      method: methodName,
      paramBytes: paramBytes,
      gas: uint64(gas),
   }

   callInfo := createRuntimeCallInfo(state, &testContext, &executeCtx);

	rt := runtime.NewRuntime(runtime.NewConfig(), logging.NewLogger("test"))
   result, err := rt.CallProgram(context.TODO(), callInfo)
   if err != nil {
      fmt.Println("Error calling program: ", err)
      // also add to the response
   }

   // TODO: feed balance
   // balance := callInfo.RemainingFuel()
   // fmt.Println("callinfo remaining")
   // grab a response
   response := C.Response{
      id: 10,
      error: nil,
      // result must be free'd by rust
      result: C.Bytes {
         data:   (*C.uint8_t)(C.CBytes(result)),
         length: C.uint(len(result)),
      },
   }

	return response
}

func createRuntimeContext(ctx *C.SimulatorCallContext) runtime.Context {
   return runtime.Context{
      Program: codec.Address(C.GoBytes(unsafe.Pointer(&ctx.programAddress), 33)),
      Actor: codec.Address(C.GoBytes(unsafe.Pointer(&ctx.actorAddress), 33)),
      Height: uint64(ctx.height),
      Timestamp: uint64(ctx.timestamp),
   }
}

func createRuntimeCallInfo(db state.Mutable, rctx *runtime.Context, e *ExecuteCtx) *runtime.CallInfo {
   return &runtime.CallInfo{
      State: simState.NewProgramStateManager(db),
      Actor: rctx.Actor,
      FunctionName: e.method,
      Program: rctx.Program,
      Params: e.paramBytes,
      Fuel: e.gas,
      Height: rctx.Height,
      Timestamp: rctx.Timestamp,
   }
}


//export CreateProgram
func CreateProgram(db *C.Mutable, path *C.char) C.CreateProgramResponse {
   state := simState.NewSimulatorState(unsafe.Pointer(db))
   programManager := simState.NewProgramStateManager(state)

   programPath := C.GoString(path)
   programBytes, err := os.ReadFile(programPath)
   if err != nil {
      return C.CreateProgramResponse {
         err: C.CString(err.Error()),
      }
   }

   programID, err := generateRandomID()
   if err != nil {
      return C.CreateProgramResponse {
         err: C.CString(err.Error()),
      }
   }

   err = programManager.SetProgram(context.TODO(), programID, programBytes)
   if err != nil {
      errmsg := fmt.Sprintf("program creation failed: %s", err.Error())
      return C.CreateProgramResponse {
         err: C.CString(errmsg),
      }
      
   }

   account, err := programManager.DeployProgram(context.TODO(), programID, []byte{})
   if err != nil {
      errmsg := fmt.Sprintf("program deployment failed: %s", err.Error())
      return C.CreateProgramResponse {
         err: C.CString(errmsg),
      }
   }
   return C.CreateProgramResponse {
      err: nil,
      programID: C.ID {
         id: *(*[32]C.char)(C.CBytes(programID[:])),
      },
      programAddress: C.Address{
         address: *(*[33]C.char)(C.CBytes(account[:])),
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


func main() {}


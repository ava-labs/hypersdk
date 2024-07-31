// include <stdint.h> required to import uint8_t
package main

/*
#cgo CFLAGS: -I./include
#include "types.h"
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
	simState "github.com/ava-labs/hypersdk/x/programs/state"
)

//export CallProgram
func CallProgram(db *C.Mutable) {
	// form db from params
	state := simState.NewSimulatorState(unsafe.Pointer(db))
	fmt.Println("Calling CallProgram")
	state.GetValue(context.TODO(), []byte{1, 2, 3})
	state.Insert(context.TODO(), []byte{1, 2, 3}, []byte{6, 6, 9})
	state.GetValue(context.TODO(), []byte{1, 2, 3})
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
func Execute(db *C.SimpleMutable, ctx *C.SimulatorContext,  p *C.ExecutionRequest) C.Response {
   // TODO: error checking making sure the params are not nil
   // db
   state := db.state;

   // ctx
   testContext := createRuntimeContext(ctx);

   // ExecutionRequest passed from the C code
   paramBytes := C.GoBytes(unsafe.Pointer(p.params), C.int(p.paramLength))
	methodName := C.GoString(p.method)
   gas := p.maxGas

   executeCtx := ExecuteCtx{
      method: methodName,
      paramBytes: paramBytes,
      gas: uint64(gas),
   }

   callInfo := createRuntimeCallInfo(nil, &testContext, &executeCtx);

	rt := runtime.NewRuntime(runtime.NewConfig(), logging.NewLogger("test"))
   result, err := rt.CallProgram(context.TODO(), callInfo)
   if err != nil {
      fmt.Println("Error calling program: ", err)
      // also add to the response
   }

   // grab a response
   response := C.Response{
      id: state,
      error: nil,
      result: nil,
   }

	fmt.Println("received bytes from C: ", paramBytes)
	fmt.Println("received method name from C: ", methodName)
	fmt.Println("Max gas: ", gas)
	fmt.Println("DB State: ", state)
	fmt.Println("Context height: ", ctx.height)
	return response
}

func createRuntimeContext(ctx *C.SimulatorContext) runtime.Context {
   return runtime.Context{
      Program: codec.Address(C.GoBytes(unsafe.Pointer(&ctx.programAddress), 33)),
      Actor: codec.Address(C.GoBytes(unsafe.Pointer(&ctx.actorAddress), 33)),
      Height: uint64(ctx.height),
      Timestamp: uint64(ctx.timestamp),
   }
}

func createRuntimeCallInfo(db *state.SimpleMutable, rctx *runtime.Context, e *ExecuteCtx) *runtime.CallInfo {
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


func main() {}

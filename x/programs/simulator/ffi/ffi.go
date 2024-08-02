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
func CallProgram(db *C.Mutable, ctx *C.SimulatorCallContext) C.CallProgramResponse {
   // TODO: error checking making sure the params are not nil
   // db
	state := simState.NewSimulatorState(unsafe.Pointer(db))
   // call info
   callInfo := createRuntimeCallInfo(state, ctx);
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
   response := C.CallProgramResponse{
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
      Program: codec.Address(C.GoBytes(unsafe.Pointer(&ctx.program_address), 33)),
      Actor: codec.Address(C.GoBytes(unsafe.Pointer(&ctx.actor_address), 33)),
      Height: uint64(ctx.height),
      Timestamp: uint64(ctx.timestamp),
   }
}

func createRuntimeCallInfo(db state.Mutable, ctx *C.SimulatorCallContext) *runtime.CallInfo {
   paramBytes := C.GoBytes(unsafe.Pointer(ctx.params.data), C.int(ctx.params.length))
	methodName := C.GoString(ctx.method)
   return &runtime.CallInfo{
      State: simState.NewProgramStateManager(db),
      Actor: codec.Address(C.GoBytes(unsafe.Pointer(&ctx.actor_address), 33)),
      FunctionName: methodName,
      Program: codec.Address(C.GoBytes(unsafe.Pointer(&ctx.program_address), 33)),
      Params: paramBytes,
      Fuel: uint64(ctx.max_gas),
      Height: uint64(ctx.height),
      Timestamp: uint64(ctx.timestamp),
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
         error: C.CString(err.Error()),
      }
   }

   programID, err := generateRandomID()
   if err != nil {
      return C.CreateProgramResponse {
         error: C.CString(err.Error()),
      }
   }

   err = programManager.SetProgram(context.TODO(), programID, programBytes)
   if err != nil {
      errmsg := fmt.Sprintf("program creation failed: %s", err.Error())
      return C.CreateProgramResponse {
         error: C.CString(errmsg),
      }
      
   }

   account, err := programManager.DeployProgram(context.TODO(), programID, []byte{})
   if err != nil {
      errmsg := fmt.Sprintf("program deployment failed: %s", err.Error())
      return C.CreateProgramResponse {
         error: C.CString(errmsg),
      }
   }
   return C.CreateProgramResponse {
      error: nil,
      program_id: C.ID {
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


func main() {}


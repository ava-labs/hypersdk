// include <stdint.h> required to import uint8_t
package main

/*
#include <stdint.h>

typedef struct {
   // View v; maybe we could use a view here but not sure?
   int state;
} SimpleMutable;

typedef struct {
    char* method;
    uint8_t* params;
    unsigned int paramLength;
    unsigned int maxGas;
} ExecutionRequest;

typedef struct {
   char address[33];
} Address;

typedef struct {
   char id[32];
} ID;

typedef struct {
   Address programAddress;
   Address actorAddress;
   unsigned int height;
   unsigned int timestamp;
} SimulatorContext;

typedef struct {
      int id;
      char* error;
      uint8_t* result;
} Response;

*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

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
   // testContext := createRuntimeContext(ctx);
   
   // ExecutionRequest passed from the C code
   paramBytes := C.GoBytes(unsafe.Pointer(p.params), C.int(p.paramLength))
	methodName := C.GoString(p.method)
   gas := p.maxGas

   // executeCtx := ExecuteCtx{
   //    method: methodName,
   //    paramBytes: paramBytes,
   //    gas: uint64(gas),
   // }


   // callInfo := createRuntimeCallInfo(nil, &testContext, &executeCtx);

	// rt := runtime.NewRuntime(runtime.NewConfig(), logging.NewLogger("test"))
   // result, err := rt.CallProgram(context.TODO(), callInfo)
   // if err != nil {
   //    fmt.Println("Error calling program: ", err)
   //    // also add to the response
   // }

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

// func createRuntimeCallInfo(db *state.SimpleMutable, rctx *runtime.Context, e *ExecuteCtx) *runtime.CallInfo {
//    return &runtime.CallInfo{
//       State: &programStateManager{Mutable: db},
//       Actor: rctx.Actor,
//       FunctionName: e.method,
//       Program: rctx.Program,
//       Params: e.paramBytes,
//       Fuel: e.gas,
//       Height: rctx.Height,
//       Timestamp: rctx.Timestamp,
//    }
// }


type programStateManager struct {
   state.Mutable
}



// func Deserialize

// func SerializeResponse(resp C.Response) []byte {
//    // serialize the response
//    return nil
// }


func main() {}

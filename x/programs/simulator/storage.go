package simulator

/*
#cgo CFLAGS: -I../ffi/include
#include "callbacks.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/ava-labs/hypersdk/state"
)

// ensure SimulatorState implements state.Mutable
var _ state.Mutable = &SimulatorState{}

type SimulatorState struct {
   // this is a pointer to the state passed in from rust
   // it points to the state object on the rust side of the simulator
   statePtr unsafe.Pointer

   // this is ptr to the get function
   getFunc GetStateCallbackType

//    // this is ptr to the insert function
//    insertFunc C.CallbackInsertFunc

//    // this is a ptr to the delete function
//    deleteFunc C.CallbackDeleteFunc
}

func NewSimulatorState(db unsafe.Pointer) *SimulatorState {
	// convert unsafe pointer to C.Mutable
	mutable := (*C.Mutable)(db)
   return &SimulatorState{
	statePtr: mutable.stateObj,
	  getFunc: mutable.get_value_callback,
   }
}

func (s *SimulatorState) GetValue(ctx context.Context, key []byte) ([]byte, error) {
	val := GetCallbackWrapper(s.getFunc, s.statePtr, key)
	fmt.Println("Value returned*******: ", val)
   return []byte{}, nil
}

func (s *SimulatorState) Insert(ctx context.Context, key []byte, value []byte) error {
	return nil
}

func (s *SimulatorState) Remove(ctx context.Context, key []byte) error {
	return  nil
}

// // ensure SimulatorStateManager implements StateManager
// var _ runtime.StateManager = &SimulatorStateManager{}

// type programStateManager struct {
//    state *SimulatorState
// }





type GetStateCallbackType = C.GetStateCallback
type Mutable = C.Mutable
// In C, a function argument written as a fixed size array actually requires a 
// pointer to the first element of the array. C compilers are aware of this calling 
// convention and adjust the call accordingly, but Go cannot. In Go, you must pass
// the pointer to the first element explicitly: C.f(&C.x[0]).
//
// getFuncPtr is a pointer to the get state callback function
// dbPtr is a pointer to the state object on the rust side of the simulator
// key is the key to be used to get the value from the state object
// this function returns the value grabbed from the state object
func GetCallbackWrapper(getFuncPtr GetStateCallbackType, dbPtr unsafe.Pointer, key []byte) []byte {
	bytesPtr := C.CBytes(key)
	defer C.free(bytesPtr)

	bytesStruct := C.Bytes{
		data: (*C.uint8_t)(bytesPtr),
		length: C.uint(len(key)),
	}
	valueBytes := C.bridge_get_callback(getFuncPtr, dbPtr, bytesStruct)
	return C.GoBytes(unsafe.Pointer(valueBytes.data), C.int(valueBytes.length))
}

package simulator

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/bridge"
)

import "C"

// ensure SimulatorState implements state.Mutable
var _ state.Mutable = &SimulatorState{}

type SimulatorState struct {
   // this is a pointer to the state passed in from rust
   // it points to the state object on the rust side of the simulator
   statePtr unsafe.Pointer

   // this is ptr to the get function
   getFunc bridge.GetStateCallbackType

//    // this is ptr to the insert function
//    insertFunc C.CallbackInsertFunc

//    // this is a ptr to the delete function
//    deleteFunc C.CallbackDeleteFunc
}

func NewSimulatorState(statePtr unsafe.Pointer, getFunc bridge.GetStateCallbackType) *SimulatorState {
   return &SimulatorState{
	statePtr: statePtr,
	  getFunc: getFunc,
   }
}

func (s *SimulatorState) GetValue(ctx context.Context, key []byte) ([]byte, error) {
	val := bridge.GetCallbackWrapper(s.getFunc, s.statePtr, key)
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



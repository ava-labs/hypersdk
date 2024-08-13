// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

/*
#cgo CFLAGS: -I../common
#include "callbacks.h"
#include <stdlib.h>
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"

	"github.com/ava-labs/avalanchego/database"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

// ensure SimulatorState implements state.Mutable
var _ state.Mutable = &SimulatorState{}

type SimulatorState struct {
	// this is a pointer to the state passed in from rust
	// it points to the state object on the rust side of the simulator
	statePtr unsafe.Pointer

	// this is ptr to the get function
	getFunc C.GetStateCallback

	// this is ptr to the insert function
	insertFunc C.InsertStateCallback

	// this is a ptr to the delete function
	removeFunc C.RemoveStateCallback
}

func NewSimulatorState(db unsafe.Pointer) *SimulatorState {
	// convert unsafe pointer to C.Mutable
	mutable := (*C.Mutable)(db)
	return &SimulatorState{
		statePtr:   mutable.stateObj,
		getFunc:    mutable.get_value_callback,
		insertFunc: mutable.insert_callback,
		removeFunc: mutable.remove_callback,
	}
}

func (s *SimulatorState) GetValue(_ context.Context, key []byte) ([]byte, error) {
	return s.getValue(key)
}

func (s *SimulatorState) Insert(_ context.Context, key []byte, value []byte) error {
	return s.insert(key, value)
}

func (s *SimulatorState) Remove(_ context.Context, key []byte) error {
	return s.remove(key)
}

// In C, a function argument written as a fixed size array actually requires a
// pointer to the first element of the array. C compilers are aware of this calling
// convention and adjust the call accordingly, but Go cannot. In Go, you must pass
// the pointer to the first element explicitly: C.f(&C.x[0]).
//
// getFuncPtr is a pointer to the get state callback function
// dbPtr is a pointer to the state object on the rust side of the simulator
// key is the key to be used to get the value from the state object
// this function returns the value grabbed from the state object
func (s *SimulatorState) getValue(key []byte) ([]byte, error) {
	bytesPtr := C.CBytes(key)
	defer C.free(bytesPtr)

	bytesStruct := C.Bytes{
		data:   (*C.uint8_t)(bytesPtr),
		length: C.uint(len(key)),
	}
	valueBytes := C.bridge_get_callback(s.getFunc, s.statePtr, bytesStruct)

	if valueBytes.error != nil {
		err := C.GoString(valueBytes.error)
		// todo: create our own errors so no need for a dependecy to avalanche go here
		if err == database.ErrNotFound.Error() {
			return nil, database.ErrNotFound
		}
		return nil, fmt.Errorf("error getting value: %s", err)
	}

	val := C.GoBytes(unsafe.Pointer(valueBytes.bytes.data), C.int(valueBytes.bytes.length))
	return val, nil
}

func (s *SimulatorState) insert(key []byte, value []byte) error {
	// this will malloc
	keyBytes := C.CBytes(key)
	defer C.free(keyBytes)

	valueBytes := C.CBytes(value)
	defer C.free(valueBytes)

	keyStruct := C.Bytes{
		data:   (*C.uint8_t)(keyBytes),
		length: C.uint(len(key)),
	}

	valueStruct := C.Bytes{
		data:   (*C.uint8_t)(valueBytes),
		length: C.uint(len(value)),
	}

	cErr := C.bridge_insert_callback(s.insertFunc, s.statePtr, keyStruct, valueStruct)
	if cErr != nil {
		return fmt.Errorf("error inserting value: %s", C.GoString(cErr))
	}
	return nil
}

func (s *SimulatorState) remove(key []byte) error {
	keyBytes := C.CBytes(key)
	defer C.free(keyBytes)

	keyStruct := C.Bytes{
		data:   (*C.uint8_t)(keyBytes),
		length: C.uint(len(key)),
	}

	cErr := C.bridge_remove_callback(s.removeFunc, s.statePtr, keyStruct)
	if cErr != nil {
		return fmt.Errorf("error removing value: %s", C.GoString(cErr))
	}
	return nil
}

type prefixedStateMutable struct {
	inner  state.Mutable
	prefix []byte
}

func (s *prefixedStateMutable) prefixKey(key []byte) (k []byte) {
	k = make([]byte, len(s.prefix)+len(key))
	copy(k, s.prefix)
	copy(k[len(s.prefix):], key)
	return
}

func (s *prefixedStateMutable) GetValue(ctx context.Context, key []byte) (value []byte, err error) {
	return s.inner.GetValue(ctx, s.prefixKey(key))
}

func (s *prefixedStateMutable) Insert(ctx context.Context, key []byte, value []byte) error {
	return s.inner.Insert(ctx, s.prefixKey(key), value)
}

func (s *prefixedStateMutable) Remove(ctx context.Context, key []byte) error {
	return s.inner.Remove(ctx, s.prefixKey(key))
}

func newAccountPrefixedMutable(account codec.Address, mutable state.Mutable) state.Mutable {
	return &prefixedStateMutable{inner: mutable, prefix: accountStateKey(account[:])}
}

func accountStateKey(key []byte) (k []byte) {
	k = make([]byte, 2+len(key))
	k[0] = accountPrefix
	copy(k[1:], key)
	k[len(k)-1] = accountStatePrefix
	return
}

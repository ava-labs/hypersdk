// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

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
	"github.com/ava-labs/hypersdk/x/contracts/runtime"

	simState "github.com/ava-labs/hypersdk/x/contracts/simulator/state"
)

var (
	ErrInvalidParam = errors.New("invalid parameter")
	SimLogger       = logging.NewLogger("Simulator")
	SimContext      = context.TODO()
)

//export CallContract
func CallContract(db *C.Mutable, ctx *C.SimulatorCallContext) C.CallContractResponse {
	if db == nil || ctx == nil {
		return newCallContractResponse(nil, 0, ErrInvalidParam)
	}

	// build the db
	state := simState.NewSimulatorState(unsafe.Pointer(db))
	// build the call info
	callInfo := createRuntimeCallInfo(state, ctx)
	config := runtime.NewConfig()
	config.SetDebugInfo(true)

	rt := runtime.NewRuntime(config, SimLogger)
	result, err := rt.CallContract(SimContext, callInfo)
	if err != nil {
		return newCallContractResponse(nil, 0, fmt.Errorf("error during runtime execution: %w", err))
	}

	fuel := callInfo.RemainingFuel()
	return newCallContractResponse(result, fuel, nil)
}

func createRuntimeCallInfo(db state.Mutable, ctx *C.SimulatorCallContext) *runtime.CallInfo {
	paramBytes := C.GoBytes(unsafe.Pointer(ctx.params.data), C.int(ctx.params.length))
	methodName := C.GoString(ctx.method)
	actorBytes := C.GoBytes(unsafe.Pointer(&ctx.actor_address), codec.AddressLen)       //nolint:all
	contractBytes := C.GoBytes(unsafe.Pointer(&ctx.contract_address), codec.AddressLen) //nolint:all

	return &runtime.CallInfo{
		State:        simState.NewContractStateManager(db),
		Actor:        codec.Address(actorBytes),
		FunctionName: methodName,
		Contract:     codec.Address(contractBytes),
		Params:       paramBytes,
		Fuel:         uint64(ctx.max_gas),
		Height:       uint64(ctx.height),
		Timestamp:    uint64(ctx.timestamp),
	}
}

//export CreateContract
func CreateContract(db *C.Mutable, path *C.char) C.CreateContractResponse {
	state := simState.NewSimulatorState(unsafe.Pointer(db))
	contractManager := simState.NewContractStateManager(state)

	contractPath := C.GoString(path)
	contractBytes, err := os.ReadFile(contractPath)
	if err != nil {
		return C.CreateContractResponse{
			error: C.CString(err.Error()),
		}
	}

	id, err := generateRandomID()
	if err != nil {
		return C.CreateContractResponse{
			error: C.CString(err.Error()),
		}
	}

	contractID := runtime.ContractID(id[:])
	err = contractManager.SetContractBytes(context.TODO(), contractID, contractBytes)
	if err != nil {
		errmsg := "contract creation failed: " + err.Error()
		return C.CreateContractResponse{
			error: C.CString(errmsg),
		}
	}

	account, err := contractManager.NewAccountWithContract(context.TODO(), contractID[:], []byte{})
	if err != nil {
		errmsg := "contract deployment failed: " + err.Error()
		return C.CreateContractResponse{
			error: C.CString(errmsg),
		}
	}
	return C.CreateContractResponse{
		error: nil,
		contract_id: C.ContractId{
			data:   (*C.uint8_t)(C.CBytes(contractID[:])), //nolint:all
			length: (C.size_t)(len(contractID[:])),
		},
		contract_address: C.Address{
			*(*[33]C.uchar)(C.CBytes(account[:])), //nolint:all
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

func newCallContractResponse(result []byte, fuel uint64, err error) C.CallContractResponse {
	var errPtr *C.char
	if err == nil {
		errPtr = nil
	} else {
		errPtr = C.CString(err.Error())
	}

	return C.CallContractResponse{
		error: errPtr,
		result: C.Bytes{
			data:   (*C.uint8_t)(C.CBytes(result)),
			length: C.size_t(len(result)),
		},
		fuel: C.uint64_t(fuel),
	}
}

// getBalance returns the balance of [account].
// Panics if there is an error.
//
//export GetBalance
func GetBalance(db *C.Mutable, address C.Address) C.uint64_t {
	if db == nil {
		panic(ErrInvalidParam)
	}

	state := simState.NewSimulatorState(unsafe.Pointer(db))
	pState := simState.NewContractStateManager(state)
	account := C.GoBytes(unsafe.Pointer(&address.address), codec.AddressLen) //nolint:all

	balance, err := pState.GetBalance(SimContext, codec.Address(account))
	if err != nil {
		panic(err)
	}

	return C.uint64_t(balance)
}

// SetBalance sets the balance of [account] to [balance].
// Panics if there is an error.
//
//export SetBalance
func SetBalance(db *C.Mutable, address C.Address, balance C.uint64_t) {
	if db == nil {
		panic(ErrInvalidParam)
	}

	state := simState.NewSimulatorState(unsafe.Pointer(db))
	pState := simState.NewContractStateManager(state)
	account := C.GoBytes(unsafe.Pointer(&address.address), codec.AddressLen) //nolint:all

	err := pState.SetBalance(SimContext, codec.Address(account), uint64(balance))
	if err != nil {
		panic(err)
	}
}

func main() {}

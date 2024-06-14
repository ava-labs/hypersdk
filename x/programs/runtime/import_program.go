// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"errors"
	"slices"

	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/hypersdk/codec"
)

type ErrorCode byte

const (
	callProgramCost = 10000
	setResultCost   = 10000
	getFuelCost     = 10000
)

const (
	CallPanicked ErrorCode = iota
	ExecutionFailure
	OutOfFuel
)

func extractErrorCode(err error) (ErrorCode, bool) {
	var trap *wasmtime.Trap
	if errors.As(err, &trap) {
		switch *trap.Code() {
		case wasmtime.UnreachableCodeReached:
			return CallPanicked, true
		case wasmtime.OutOfFuel:
			return OutOfFuel, true
		default:
			return ExecutionFailure, true
		}
	}
	return 0, false
}

type callProgramInput struct {
	Program      codec.Address
	FunctionName string
	Params       []byte
	Fuel         uint64
}

func NewProgramModule(r *WasmRuntime) *ImportModule {
	return &ImportModule{
		Name: "program",
		HostFunctions: map[string]HostFunction{
			"call_program": {FuelCost: callProgramCost, Function: Function[callProgramInput, Result[RawBytes, ErrorCode]](func(callInfo *CallInfo, input callProgramInput) (Result[RawBytes, ErrorCode], error) {
				newInfo := *callInfo

				if err := callInfo.ConsumeFuel(input.Fuel); err != nil {
					return Err[RawBytes, ErrorCode](OutOfFuel), nil
				}

				newInfo.Actor = callInfo.Program
				newInfo.Program = input.Program
				newInfo.FunctionName = input.FunctionName
				newInfo.Params = input.Params
				newInfo.Fuel = input.Fuel

				result, err := r.CallProgram(
					context.Background(),
					&newInfo)
				if err != nil {
					if code, ok := extractErrorCode(err); ok {
						return Err[RawBytes, ErrorCode](code), nil
					}
					return Err[RawBytes, ErrorCode](ExecutionFailure), err
				}

				// return any remaining fuel to the calling program
				callInfo.AddFuel(newInfo.RemainingFuel())

				return Ok[RawBytes, ErrorCode](result), nil
			})},
			"set_call_result": {FuelCost: setResultCost, Function: FunctionNoOutput[RawBytes](func(callInfo *CallInfo, input RawBytes) error {
				// needs to clone because this points into the current store's linear memory which may be gone when this is read
				callInfo.inst.result = slices.Clone(input)
				return nil
			})},
			"remaining_fuel": {FuelCost: getFuelCost, Function: FunctionNoInput[uint64](func(callInfo *CallInfo) (uint64, error) {
				return callInfo.RemainingFuel(), nil
			})},
		},
	}
}

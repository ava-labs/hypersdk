// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"errors"
	"slices"

	"github.com/ava-labs/hypersdk/codec"
)

const (
	callProgramCost = 10000
	setResultCost   = 10000
	getFuelCost     = 10000
)

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
			"call_program": {FuelCost: callProgramCost, Function: Function[callProgramInput, RawBytes](func(callInfo *CallInfo, input callProgramInput) (RawBytes, error) {
				newInfo := *callInfo

				// make sure there is enough fuel in current store to give to the new call
				if callInfo.RemainingFuel() < input.Fuel {
					return nil, errors.New("remaining fuel is less than requested fuel")
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
					return nil, err
				}

				// subtract the fuel used during this call from the calling program
				remainingFuel := newInfo.RemainingFuel()
				if err := callInfo.ConsumeFuel(input.Fuel - remainingFuel); err != nil {
					return nil, err
				}

				return result, nil
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

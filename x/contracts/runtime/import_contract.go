// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"errors"
	"slices"

	"github.com/bytecodealliance/wasmtime-go/v25"

	"github.com/ava-labs/hypersdk/codec"
)

type ContractCallErrorCode byte

const (
	callContractCost  = 10000
	setCallResultCost = 10000
	remainingFuelCost = 10000
	deployCost        = 10000
)

const (
	ExecutionFailure ContractCallErrorCode = iota
	CallPanicked
	OutOfFuel
	InsufficientBalance
)

type callContractInput struct {
	Contract     codec.Address
	FunctionName string
	Params       []byte
	Fuel         uint64
	Value        uint64
}

type deployContractInput struct {
	ContractID          ContractID
	AccountCreationData []byte
}

func ExtractContractCallErrorCode(err error) (ContractCallErrorCode, bool) {
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

func NewContractModule(r *WasmRuntime) *ImportModule {
	return &ImportModule{
		Name: "contract",
		HostFunctions: map[string]HostFunction{
			"call_contract": {FuelCost: callContractCost, Function: Function[callContractInput, Result[RawBytes, ContractCallErrorCode]](func(callInfo *CallInfo, input callContractInput) (Result[RawBytes, ContractCallErrorCode], error) {
				newInfo := *callInfo

				if err := callInfo.ConsumeFuel(input.Fuel); err != nil {
					return Err[RawBytes, ContractCallErrorCode](OutOfFuel), nil //nolint:nilerr
				}

				newInfo.Actor = callInfo.Contract
				newInfo.Contract = input.Contract
				newInfo.FunctionName = input.FunctionName
				newInfo.Params = input.Params
				newInfo.Fuel = input.Fuel
				newInfo.Value = input.Value

				result, err := r.CallContract(
					context.Background(),
					&newInfo)
				if err != nil {
					if code, ok := ExtractContractCallErrorCode(err); ok {
						return Err[RawBytes, ContractCallErrorCode](code), nil
					}
					return Err[RawBytes, ContractCallErrorCode](ExecutionFailure), err
				}

				// return any remaining fuel to the calling contract
				callInfo.AddFuel(newInfo.RemainingFuel())

				return Ok[RawBytes, ContractCallErrorCode](result), nil
			})},
			"set_call_result": {FuelCost: setCallResultCost, Function: FunctionNoOutput[RawBytes](func(callInfo *CallInfo, input RawBytes) error {
				// needs to clone because this points into the current store's linear memory which may be gone when this is read
				callInfo.inst.result = slices.Clone(input)
				return nil
			})},
			"remaining_fuel": {FuelCost: remainingFuelCost, Function: FunctionNoInput[uint64](func(callInfo *CallInfo) (uint64, error) {
				return callInfo.RemainingFuel(), nil
			})},
			"deploy": {
				FuelCost: deployCost,
				Function: Function[deployContractInput, codec.Address](
					func(callInfo *CallInfo, input deployContractInput) (codec.Address, error) {
						ctx, cancel := context.WithCancel(context.Background())
						defer cancel()
						address, err := callInfo.State.NewAccountWithContract(ctx, input.ContractID, input.AccountCreationData)
						if err != nil {
							return codec.EmptyAddress, err
						}
						return address, nil
					}),
			},
		},
	}
}

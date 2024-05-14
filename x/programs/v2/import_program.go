package v2

import (
	"context"
	"errors"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/near/borsh-go"
)

type callProgramInput struct {
	ProgramID    []byte
	FunctionName []byte
	Params       []byte
	Fuel         uint64
}

type setResultInput struct {
	Offset int32
	Length int32
}

func NewCallProgramModule(r *WasmRuntime) *ImportModule {
	return &ImportModule{name: "program",
		funcs: map[string]HostFunction{
			"call_program": FunctionWithOutput(func(callInfo *CallInfo, input []byte) ([]byte, error) {
				newInfo := *callInfo
				parsedInput := &callProgramInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}

				newInfo.ProgramID = ids.ID(parsedInput.ProgramID)
				newInfo.FunctionName = string(parsedInput.FunctionName)
				newInfo.Params = parsedInput.Params
				newInfo.Fuel = parsedInput.Fuel

				if callInfo.RemainingFuel() < parsedInput.Fuel {
					return nil, errors.New("remaining fuel is less than requested fuel")
				}
				// make sure there is enough fuel in current store to give
				result, err := r.CallProgram(
					context.Background(),
					&newInfo)
				remainingFuel := newInfo.RemainingFuel()
				callInfo.ConsumeFuel(parsedInput.Fuel - remainingFuel)
				return result, err
			}),
			"set_call_result": FunctionNoOutput(func(callInfo *CallInfo, input []byte) error {
				callInfo.inst.result = input
				return nil
			}),
		},
	}
}

package v2

import (
	"context"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/near/borsh-go"
)

type callInput struct {
	ProgramAccount ids.ID
	ProgramID      ids.ID
	FunctionName   string
	Params         []byte
}

type setResultInput struct {
	Offset int32
	Length int32
}

func NewCallProgramModule(r *WasmRuntime) *ImportModule {
	return &ImportModule{name: "program",
		funcs: map[string]HostFunction{
			"call": FunctionWithOutput(func(callInfo *CallInfo, input []byte) ([]byte, error) {
				newInfo := *callInfo
				parsedInput := &callInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}

				newInfo.ProgramID = parsedInput.ProgramID
				newInfo.Account = parsedInput.ProgramAccount
				newInfo.FunctionName = parsedInput.FunctionName
				newInfo.Params = parsedInput.Params

				return r.CallProgram(
					context.Background(),
					&newInfo)
			}),
			"set_call_result": FunctionNoOutput(func(callInfo *CallInfo, input []byte) error {
				callInfo.result = input
				return nil
			}),
		},
	}
}

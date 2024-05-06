package v2

import (
	"context"
	"github.com/near/borsh-go"
)

type callInput struct {
	ProgramAccount AccountID
	ProgramID      ProgramID
	FunctionName   string
	Params         []byte
}

func NewCallProgramModule(r *WasmRuntime) *ImportModule {
	return &ImportModule{name: "program",
		funcs: map[string]Function{
			"call": func(callInfo *CallInfo, input []byte) ([]byte, error) {
				newInfo := *callInfo
				parsedInput := &callInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}

				newInfo.Account = parsedInput.ProgramAccount
				newInfo.Program = parsedInput.ProgramID
				newInfo.FunctionName = parsedInput.FunctionName
				newInfo.Params = parsedInput.Params

				return r.CallProgram(
					context.Background(),
					&newInfo)
			},
		},
	}
}

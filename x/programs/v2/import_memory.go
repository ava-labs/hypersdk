package v2

import (
	"github.com/near/borsh-go"
)

type setResultInput struct {
	Offset int32
	Length int32
}

func NewMemoryModule() *ImportModule {
	return &ImportModule{name: "memory",
		funcs: map[string]Function{
			"set_result": func(callInfo *CallInfo, input []byte) ([]byte, error) {
				parsedInput := &setResultInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}
				return nil, callInfo.programInstance.setResult(parsedInput.Offset, parsedInput.Length)
			},
		},
	}
}

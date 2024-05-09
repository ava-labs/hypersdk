package v2

import (
	"context"
	"errors"
	"github.com/near/borsh-go"
)

func NewMemoryModule(r *WasmRuntime) *ImportModule {
	return &ImportModule{name: "memory",
		funcs: map[string]Function{
			"setResult": func(callInfo *CallInfo, input []byte) ([]byte, error) {
				parsedInput := &keyInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}
				// key is relative to current account
				readKey := []byte(callInfo.Account.String() + "/" + string(parsedInput.Key))
				if !callInfo.StateAccessList.CanRead(readKey) {
					return nil, errors.New("can only read from specified keys")
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return callInfo.State.GetValue(ctx, readKey)
			},
		},
	}
}

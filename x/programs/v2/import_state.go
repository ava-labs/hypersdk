package v2

import (
	"context"
	"errors"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/hypersdk/state"
	"github.com/near/borsh-go"
)

type StateAccessModule struct {
	ImportModule
	mu state.Mutable
}

type keyInput struct {
	Key []byte
}

type keyValueInput struct {
	Key   []byte
	Value []byte
}

func NewStateAccessModule() *ImportModule {
	return &ImportModule{name: "state",
		funcs: map[string]HostFunction{
			"get": FunctionWithOutput(func(callInfo *CallInfo, input []byte) ([]byte, error) {
				parsedInput := &keyInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}
				// key is relative to current account
				readKey := []byte(callInfo.Account.String() + "/" + string(parsedInput.Key))
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				val, err := callInfo.State.GetValue(ctx, readKey)
				if err != nil {
					if errors.Is(err, database.ErrNotFound) {
						return nil, nil
					}
					return nil, err
				}
				return val, nil
			}),
			"put": FunctionWithOutput(func(callInfo *CallInfo, input []byte) ([]byte, error) {
				parsedInput := &keyValueInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}
				// key is relative to current account
				writeKey := []byte(callInfo.Account.String() + "/" + string(parsedInput.Key))
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return nil, callInfo.State.Insert(ctx, writeKey, parsedInput.Value)
			}),
			"delete": FunctionWithOutput(func(callInfo *CallInfo, input []byte) ([]byte, error) {
				parsedInput := &keyInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}

				// key is relative to current account
				writeKey := []byte(callInfo.Account.String() + "/" + string(parsedInput.Key))
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return nil, callInfo.State.Remove(ctx, writeKey)
			}),
			"log": FunctionNoOutput(func(callInfo *CallInfo, input []byte) error {
				return log("INFO", input)
			}),
		},
	}
}

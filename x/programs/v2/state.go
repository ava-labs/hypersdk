package v2

import (
	"context"
	"errors"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/state"
	"github.com/near/borsh-go"
)

type StateAccessModule struct {
	ImportModule
	mu     state.Mutable
	saList StateAccessList
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
		funcs: map[string]Function{
			"get": func(callInfo *CallInfo, input []byte) ([]byte, error) {
				parsedInput := &keyInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}
				if !callInfo.StateAccessList.CanRead(parsedInput.Key) {
					return nil, errors.New("can only read from specified keys")
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return callInfo.State.GetValue(ctx, parsedInput.Key)
			},
			"put": func(callInfo *CallInfo, input []byte) ([]byte, error) {
				parsedInput := &keyValueInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}
				if !callInfo.StateAccessList.CanWrite(parsedInput.Key) {
					return nil, errors.New("can only write to specified keys")
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return nil, callInfo.State.Insert(ctx, parsedInput.Key, parsedInput.Value)
			},
			"delete": func(callInfo *CallInfo, input []byte) ([]byte, error) {
				parsedInput := &keyInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}
				if !callInfo.StateAccessList.CanWrite(parsedInput.Key) {
					return nil, errors.New("can only write to specified keys")
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return nil, callInfo.State.Remove(ctx, parsedInput.Key)
			},
		},
	}
}

type StateAccessList struct {
	read  set.Set[string]
	write set.Set[string]
}

func (saList *StateAccessList) CanRead(key []byte) bool {
	return saList.read.Contains(string(key))
}

func (saList *StateAccessList) CanWrite(key []byte) bool {
	return saList.write.Contains(string(key))
}

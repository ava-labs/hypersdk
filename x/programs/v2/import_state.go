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
		funcs: map[string]HostFunction{
			"get": FunctionWithOutput(func(callInfo *CallInfo, input []byte) ([]byte, error) {
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
			}),
			"put": FunctionNoOutput(func(callInfo *CallInfo, input []byte) error {
				parsedInput := &keyValueInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return err
				}
				// key is relative to current account
				writeKey := []byte(callInfo.Account.String() + "/" + string(parsedInput.Key))
				if !callInfo.StateAccessList.CanWrite(writeKey) {
					return errors.New("can only write to specified keys")
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return callInfo.State.Insert(ctx, writeKey, parsedInput.Value)
			}),
			"delete": FunctionNoOutput(func(callInfo *CallInfo, input []byte) error {
				parsedInput := &keyInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return err
				}

				// key is relative to current account
				writeKey := []byte(callInfo.Account.String() + "/" + string(parsedInput.Key))
				if !callInfo.StateAccessList.CanWrite(writeKey) {
					return errors.New("can only write to specified keys")
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return callInfo.State.Remove(ctx, writeKey)
			}),
		},
	}
}

type StateAccessList interface {
	CanRead(key []byte) bool
	CanWrite(key []byte) bool
}

type stateAccessList struct {
	read  set.Set[string]
	write set.Set[string]
}

func NewSimpleStateAccessList(readKeys [][]byte, writeKeys [][]byte) StateAccessList {
	result := &stateAccessList{}
	result.read = set.NewSet[string](len(readKeys))
	for _, key := range readKeys {
		result.read.Add(string(key))
	}

	result.write = set.NewSet[string](len(writeKeys))
	for _, key := range writeKeys {
		result.write.Add(string(key))
	}
	return result
}

func (saList *stateAccessList) CanRead(key []byte) bool {
	return true
	//return saList.read.Contains(string(key))
}

func (saList *stateAccessList) CanWrite(key []byte) bool {
	return true
	//return saList.write.Contains(string(key))
}

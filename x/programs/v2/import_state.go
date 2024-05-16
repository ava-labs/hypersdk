// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package v2

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/near/borsh-go"
)

type keyInput struct {
	Key []byte
}

type keyValueInput struct {
	Key   []byte
	Value []byte
}

// prependAccountToKey makes the key relative to the account
func prependAccountToKey(account ids.ID, key []byte) []byte {
	return []byte(account.String() + "/" + string(key))
}

func NewStateAccessModule() *ImportModule {
	return &ImportModule{
		name: "state",
		funcs: map[string]HostFunction{
			"get": FunctionWithOutput(func(callInfo *CallInfo, input []byte) ([]byte, error) {
				parsedInput := &keyInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				val, err := callInfo.State.GetValue(ctx, prependAccountToKey(callInfo.Account, parsedInput.Key))
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
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return nil, callInfo.State.Insert(ctx, prependAccountToKey(callInfo.Account, parsedInput.Key), parsedInput.Value)
			}),
			"delete": FunctionWithOutput(func(callInfo *CallInfo, input []byte) ([]byte, error) {
				parsedInput := &keyInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return nil, callInfo.State.Remove(ctx, prependAccountToKey(callInfo.Account, parsedInput.Key))
			}),
		},
	}
}

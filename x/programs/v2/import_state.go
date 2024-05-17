// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package v2

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/near/borsh-go"

	"github.com/ava-labs/hypersdk/codec"
)

const (
	readCost   = 10000
	writeCost  = 10000
	deleteCost = 10000
)

type keyInput struct {
	Key []byte
}

type keyValueInput struct {
	Key   []byte
	Value []byte
}

// prependAccountToKey makes the key relative to the account
func prependAccountToKey(account codec.Address, key []byte) []byte {
	result := make([]byte, len(account)+len(key)+1)
	copy(result, account[:])
	copy(result[len(account):], "/")
	copy(result[len(account)+1:], key)
	return result
}

func NewStateAccessModule() *ImportModule {
	return &ImportModule{
		Name: "state",
		HostFunctions: map[string]HostFunction{
			"get": {FuelCost: readCost, Function: FunctionWithOutput(func(callInfo *CallInfo, input []byte) ([]byte, error) {
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
			})},
			"put": {FuelCost: writeCost, Function: FunctionWithOutput(func(callInfo *CallInfo, input []byte) ([]byte, error) {
				parsedInput := &keyValueInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return nil, callInfo.State.Insert(ctx, prependAccountToKey(callInfo.Account, parsedInput.Key), parsedInput.Value)
			})},
			"delete": {FuelCost: deleteCost, Function: FunctionWithOutput(func(callInfo *CallInfo, input []byte) ([]byte, error) {
				parsedInput := &keyInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return nil, callInfo.State.Remove(ctx, prependAccountToKey(callInfo.Account, parsedInput.Key))
			})},
		},
	}
}

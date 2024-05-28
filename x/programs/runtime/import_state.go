// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

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
			"get": {FuelCost: readCost, Function: Function(func(callInfo *CallInfo, input []byte) ([]byte, error) {
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
			"put": {FuelCost: writeCost, Function: FunctionNoOutput(func(callInfo *CallInfo, input []byte) error {
				parsedInput := &keyValueInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return err
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return callInfo.State.Insert(ctx, prependAccountToKey(callInfo.Account, parsedInput.Key), parsedInput.Value)
			})},
			"delete": {FuelCost: deleteCost, Function: Function(func(callInfo *CallInfo, input []byte) ([]byte, error) {
				parsedInput := &keyInput{}
				if err := borsh.Deserialize(parsedInput, input); err != nil {
					return nil, err
				}

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				key := prependAccountToKey(callInfo.Account, parsedInput.Key)
				bytes, err := callInfo.State.GetValue(ctx, key)
				if err != nil {
					if errors.Is(err, database.ErrNotFound) {
						// [0] represents `None`
						return []byte{0}, nil
					}

					return nil, err
				}

				bytes = append([]byte{1}, bytes...)

				err = callInfo.State.Remove(ctx, key)
				if err != nil {
					return nil, err
				}

				return bytes, nil
			})},
		},
	}
}

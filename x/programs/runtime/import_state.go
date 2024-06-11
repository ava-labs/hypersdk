// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
	"github.com/near/borsh-go"
)

const (
	getCost    = 10000
	putCost    = 10000
	deleteCost = 10000

	putManyCost = 10000
)

type keyValueInput struct {
	Key   []byte
	Value []byte
}

func NewStateAccessModule() *ImportModule {
	return &ImportModule{
		Name: "state",
		HostFunctions: map[string]HostFunction{
			"get": {FuelCost: getCost, Function: Function[[]byte, []byte](func(callInfo *CallInfo, input []byte) ([]byte, error) {
				var parsedInput []byte
				if err := borsh.Deserialize(&parsedInput, input); err != nil {
					return nil, err
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				val, err := callInfo.State.GetProgramState(callInfo.Program).GetValue(ctx, parsedInput)
				if err != nil {
					if errors.Is(err, database.ErrNotFound) {
						return nil, nil
					}
					return nil, err
				}
				return val, nil
			})},
			"put": {FuelCost: putCost, Function: FunctionNoOutput[keyValueInput](func(callInfo *CallInfo, input keyValueInput) error {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return callInfo.State.GetProgramState(callInfo.Program).Insert(ctx, input.Key, input.Value)
			})},
			"put_many": {FuelCost: putManyCost, Function: FunctionNoOutput[[]keyValueInput](func(callInfo *CallInfo, input []keyValueInput) error {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				for _, entry := range input {
					if err := callInfo.State.GetProgramState(callInfo.Program).Insert(ctx, entry.Key, entry.Value); err != nil {
						return err
					}
				}
				return nil
			})},
			"delete": {FuelCost: deleteCost, Function: Function[[]byte, []byte](func(callInfo *CallInfo, input []byte) ([]byte, error) {
				var parsedInput []byte
				if err := borsh.Deserialize(&parsedInput, input); err != nil {
					return nil, err
				}

				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				programState := callInfo.State.GetProgramState(callInfo.Program)
				bytes, err := programState.GetValue(ctx, parsedInput)
				if err != nil {
					if errors.Is(err, database.ErrNotFound) {
						return nil, nil
					}

					return nil, err
				}

				err = programState.Remove(ctx, parsedInput)
				if err != nil {
					return nil, err
				}

				return bytes, nil
			})},
		},
	}
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/database"
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
			"get": {FuelCost: getCost, Function: Function[[]byte, RawBytes](func(callInfo *CallInfo, input []byte) (RawBytes, error) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				val, err := callInfo.State.GetContractState(callInfo.Contract).GetValue(ctx, input)
				if err != nil {
					if errors.Is(err, database.ErrNotFound) {
						return nil, nil
					}
					return nil, err
				}
				return val, nil
			})},
			"put": {FuelCost: putManyCost, Function: FunctionNoOutput[[]keyValueInput](func(callInfo *CallInfo, input []keyValueInput) error {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				contractState := callInfo.State.GetContractState(callInfo.Contract)
				for _, entry := range input {
					if len(entry.Value) == 0 {
						if err := contractState.Remove(ctx, entry.Key); err != nil {
							return err
						}
					} else if err := contractState.Insert(ctx, entry.Key, entry.Value); err != nil {
						return err
					}
				}
				return nil
			})},
		},
	}
}

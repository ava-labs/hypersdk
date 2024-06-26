// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"github.com/ava-labs/hypersdk/codec"
)

const (
	transferBalanceCost = 10000
	getBalanceCost      = 10000
)

type transferBalanceInput struct {
	From   codec.Address
	To     codec.Address
	Amount uint64
}

func NewBalanceModule() *ImportModule {
	return &ImportModule{
		Name: "balance",
		HostFunctions: map[string]HostFunction{
			"get": {FuelCost: getBalanceCost, Function: Function[codec.Address, uint64](func(callInfo *CallInfo, address codec.Address) (uint64, error) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return callInfo.State.GetBalance(ctx, address)
			})},
			"transfer": {FuelCost: transferBalanceCost, Function: FunctionNoOutput[transferBalanceInput](func(callInfo *CallInfo, input transferBalanceInput) error {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				return callInfo.State.TransferBalance(ctx, input.From, input.To, input.Amount)
			})},
		},
	}
}

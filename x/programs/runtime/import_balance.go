// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"

	"github.com/ava-labs/hypersdk/codec"
)

const (
	sendBalanceCost = 10000
	getBalanceCost  = 10000
)

type transferBalanceInput struct {
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
			"send": {FuelCost: sendBalanceCost, Function: Function[transferBalanceInput, Result[Unit, ProgramCallErrorCode]](func(callInfo *CallInfo, input transferBalanceInput) (Result[Unit, ProgramCallErrorCode], error) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				err := callInfo.State.TransferBalance(ctx, callInfo.Program, input.To, input.Amount)
				if err != nil {
					if extractedError, ok := ExtractProgramCallErrorCode(err); ok {
						return Err[Unit, ProgramCallErrorCode](extractedError), nil
					}
					return Err[Unit, ProgramCallErrorCode](ExecutionFailure), err
				}
				return Ok[Unit, ProgramCallErrorCode](Unit{}), nil
			})},
		},
	}
}

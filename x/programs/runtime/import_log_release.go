// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//go:build !debug

package runtime

const logCost = 1000

func NewLogModule() *ImportModule {
	return &ImportModule{
		Name: "log",
		HostFunctions: map[string]HostFunction{
			"write": {FuelCost: logCost, Function: FunctionNoOutput[RawBytes](func(*CallInfo, RawBytes) error { return nil })},
		},
	}
}

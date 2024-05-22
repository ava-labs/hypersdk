// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"fmt"
	"os"
)

const logCost = 1000

func NewLogModule() *ImportModule {
	return &ImportModule{
		Name: "log",
		HostFunctions: map[string]HostFunction{
			"write": {FuelCost: logCost, Function: FunctionNoOutput(func(_ *CallInfo, input []byte) error {
				_, err := fmt.Fprintf(os.Stderr, "%s\n", input)
				return err
			})},
		},
	}
}

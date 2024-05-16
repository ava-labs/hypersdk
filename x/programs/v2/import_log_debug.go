// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//go:build debug

package v2

import (
	"fmt"
	"os"
)

func NewLogModule() *ImportModule {
	return &ImportModule{
		name: "log",
		funcs: map[string]HostFunction{
			"write": FunctionNoOutput(func(_ *CallInfo, input []byte) error {
				_, err := fmt.Fprintf(os.Stderr, "%s\n", input)
				return err
			}),
		},
	}
}

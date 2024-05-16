// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

//go:build !debug

package v2

func NewLogModule() *ImportModule {
	return &ImportModule{
		name: "log",
		funcs: map[string]HostFunction{
			"write": FunctionNoOutput(func(*CallInfo, []byte) error { return nil }),
		},
	}
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import "github.com/bytecodealliance/wasmtime-go/v14"

type Instance interface {
	// GetFunc returns a function exported by the program.
	GetFunc(name string) (*Func, error)
	// GetExport returns an export exported by the program.
	GetExport(name string) (*Export, error)
	// Memory returns the memory exported by the program.
	Memory() (*Memory, error)
	GetStore() wasmtime.Storelike
}

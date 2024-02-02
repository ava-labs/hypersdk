// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

var _ Instance = (*Caller)(nil)

// Caller is a wrapper around a wasmtime.Caller
type Caller struct {
	caller *wasmtime.Caller
}

func (c *Caller) GetStore() wasmtime.Storelike {
	return c.caller
}

// NewCaller creates a new program instance.
func NewCaller(caller *wasmtime.Caller) *Caller {
	return &Caller{
		caller: caller,
	}
}

func (c *Caller) GetFunc(name string) (*Func, error) {
	exp, err := c.GetExport(name)
	if err != nil {
		return nil, err
	}

	fn, err := exp.Func()
	if err != nil {
		return nil, err
	}

	return NewFunc(fn, c), nil
}

func (c *Caller) GetExport(name string) (*Export, error) {
	exp := c.caller.GetExport(FuncName(name))
	if exp == nil {
		return nil, fmt.Errorf("failed to create export %w: %s", ErrInvalidType, name)
	}

	return NewExport(exp), nil
}

func (c *Caller) Memory() (*Memory, error) {
	exp, err := c.GetExport(MemoryFnName)
	if err != nil {
		return nil, err
	}

	mem, err := exp.Memory()
	if err != nil {
		return nil, err
	}

	exp, err = c.GetExport(AllocFnName)
	if err != nil {
		return nil, err
	}

	allocFn, err := exp.Func()
	if err != nil {
		return nil, err
	}

	return NewMemory(mem, allocFn, c.caller), nil
}

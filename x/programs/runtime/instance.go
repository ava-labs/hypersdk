// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/program"
)

var _ program.Instance = (*Instance)(nil)

// NewInstance creates a new instance wrapper.
func NewInstance(store *engine.Store, inner *wasmtime.Instance) *Instance {
	return &Instance{
		inner: inner,
		store: store.Get(),
	}
}

// Instance is a wrapper around a wasmtime.Instance
type Instance struct {
	inner *wasmtime.Instance
	store wasmtime.Storelike
}

func (i *Instance) GetStore() wasmtime.Storelike {
	return i.store
}

func (i *Instance) GetFunc(name string) (*program.Func, error) {
	exp, err := i.GetExport(program.FuncName(name))
	if err != nil {
		return nil, err
	}

	fn, err := exp.Func()
	if err != nil {
		return nil, err
	}

	return program.NewFunc(fn, i), nil
}

func (i *Instance) GetExport(name string) (*program.Export, error) {
	exp := i.inner.GetExport(i.store, name)
	if exp == nil {
		return nil, fmt.Errorf("failed to create export %w: %s", program.ErrInvalidType, name)
	}

	return program.NewExport(exp), nil
}

func (i *Instance) Memory() (*program.Memory, error) {
	exp, err := i.GetExport(program.MemoryFnName)
	if err != nil {
		return nil, err
	}

	mem, err := exp.Memory()
	if err != nil {
		return nil, err
	}

	exp, err = i.GetExport(program.AllocFnName)
	if err != nil {
		return nil, err
	}

	allocFn, err := exp.Func()
	if err != nil {
		return nil, err
	}

	return program.NewMemory(mem, allocFn, i.store), nil
}

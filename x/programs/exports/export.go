// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package exports

import (
	"errors"
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

var (
	ErrInvalidType = errors.New("invalid type")
)

func New(export *wasmtime.Extern) *Export {
	return &Export{
		inner: export,
	}
}

type Export struct {
	inner *wasmtime.Extern
}

func (e *Export) Function() (*wasmtime.Func, error) {
	fn := e.inner.Func()
	if fn == nil {
		return nil, fmt.Errorf("%w: function not found", ErrInvalidType)
	}
	return fn, nil
}

func (e *Export) Global() (*wasmtime.Global, error) {
	g := e.inner.Global()
	if g == nil {
		return nil, fmt.Errorf("%w: global not found", ErrInvalidType)
	}
	return g, nil
}

func (e *Export) Memory() (*wasmtime.Memory, error) {
	m := e.inner.Memory()
	if m == nil {
		return nil, fmt.Errorf("%w: memory not found", ErrInvalidType)
	}
	return m, nil
}

func (e *Export) Table() (*wasmtime.Table, error) {
	t := e.inner.Table()
	if t == nil {
		return nil, fmt.Errorf("%w: table not found", ErrInvalidType)
	}
	return t, nil
}

// TODO: add Type() method

// NewCaller creates a new Caller which can interact with a Wasm program.
func NewCaller(caller *wasmtime.Caller) *Caller {
	return &Caller{
		inner: caller,
	}
}

type Caller struct {
	inner *wasmtime.Caller
}

func (c *Caller) GetExport(name string) (*Export, error) {
	ext := c.inner.GetExport(name)
	if ext == nil {
		return nil, fmt.Errorf("%w: export not found: %s", ErrInvalidType, name)
	}
	return New(ext), nil
}

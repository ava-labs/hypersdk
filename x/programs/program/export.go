// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

// NewExport creates a new export wrapper.
func NewExport(inner *wasmtime.Extern) *Export {
	return &Export{
		inner: inner,
	}
}

// Export is a wrapper around a wasmtime.Extern
type Export struct {
	inner *wasmtime.Extern
}

func (e *Export) Func() (*wasmtime.Func, error) {
	fn := e.inner.Func()
	if fn == nil {
		return nil, fmt.Errorf("export is not a function: %w", ErrInvalidType)
	}
	return fn, nil
}

func (e *Export) Global() (*wasmtime.Global, error) {
	gbl := e.inner.Global()
	if gbl == nil {
		return nil, fmt.Errorf("export is not a global: %w", ErrInvalidType)
	}
	return gbl, nil
}

func (e *Export) Memory() (*wasmtime.Memory, error) {
	mem := e.inner.Memory()
	if mem == nil {
		return nil, fmt.Errorf("export is not a memory: %w", ErrInvalidType)
	}
	return mem, nil
}

func (e *Export) Table() (*wasmtime.Table, error) {
	tbl := e.inner.Table()
	if tbl == nil {
		return nil, fmt.Errorf("export is not a table: %w", ErrInvalidType)
	}
	return tbl, nil
}

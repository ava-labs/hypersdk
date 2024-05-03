// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package wrap

import (
	"errors"

	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/program"
	"github.com/ava-labs/hypersdk/x/programs/program/types"
)

// New returns a new import wrap helper.
func New(link *host.Link) *Wrap {
	return &Wrap{
		link: link,
	}
}

// Wrap is a helper for registering import functions.
type Wrap struct {
	link *host.Link
}

// RegisterAnyParamFn is a helper method for registering a function with one int64 parameter.
func (w *Wrap) RegisterAnyParamFn(name, module string, paramCount int, fn AnyParamFn) error {
	return w.link.RegisterImportWrapFn(name, module, paramCount, NewImportFn[AnyParamFn](fn))
}

// AnyParamFn is a generic type that satisfies AnyParamFnType
type AnyParamFn func(*program.Caller, ...int32) (*types.Val, error)

// ImportFn is a generic type that satisfies ImportFnType
type ImportFn[F AnyParamFn] struct {
	fn F
}

// Invoke calls the underlying function with the given arguments. Currently only
// supports int32 arguments and return values.
func (i ImportFn[F]) Invoke(c *program.Caller, args ...int32) (*types.Val, error) {
	switch fn := any(i.fn).(type) {
	case AnyParamFn:
		return fn.Call(c, args...)
	default:
		return nil, errors.New("unsupported function type")
	}
}

func (fn AnyParamFn) Call(c *program.Caller, args ...int32) (*types.Val, error) {
	return fn(c, args...)
}

func NewImportFn[F AnyParamFn](src F) func(caller *program.Caller, wargs ...wasmtime.Val) (*types.Val, error) {
	importFn := ImportFn[F]{fn: src}
	fn := func(c *program.Caller, wargs ...wasmtime.Val) (*types.Val, error) {
		args := make([]int32, 0, len(wargs))
		for _, arg := range wargs {
			args = append(args, arg.I32())
		}
		return importFn.Invoke(c, args...)
	}
	return fn
}

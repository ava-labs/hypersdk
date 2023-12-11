// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package host

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/hypersdk/x/programs/program"
)

// ImportFn is a generic type that satisfies ImportFnType
type ImportFn[F any] struct {
	fn F
}

// Invoke calls the underlying function with the given arguments. Currently only
// supports int64 arguments and return values.
func (i ImportFn[F]) Invoke(c *program.Caller, args ...int64) (*program.Val, error) {
	switch fn := any(i.fn).(type) {
	case OneParam:
		return fn.Call(c, args[0])
	case TwoParam:
		return fn.Call(c, args[0], args[1])
	case ThreeParam:
		return fn.Call(c, args[0], args[1], args[2])
	case FourParam:
		return fn.Call(c, args[0], args[1], args[2], args[3])
	case FiveParam:
		return fn.Call(c, args[0], args[1], args[2], args[3], args[4])
	case SixParam:
		return fn.Call(c, args[0], args[1], args[2], args[3], args[4], args[5])
	default:
		return nil, fmt.Errorf("unsupported")
	}
}

type OneParamFn func(*program.Caller, int64) (*program.Val, error)

func (fn OneParamFn) Call(c *program.Caller, arg1 int64) (*program.Val, error) {
	return fn(c, arg1)
}

type TwoParamFn func(*program.Caller, int64, int64) (*program.Val, error)

func (fn TwoParamFn) Call(c *program.Caller, arg1, arg2 int64) (*program.Val, error) {
	return fn(c, arg1, arg2)
}

type ThreeParamFn func(*program.Caller, int64, int64, int64) (*program.Val, error)

func (fn ThreeParamFn) Call(c *program.Caller, arg1, arg2, arg3 int64) (*program.Val, error) {
	return fn(c, arg1, arg2, arg3)
}

type FourParamFn func(*program.Caller, int64, int64, int64, int64) (*program.Val, error)

func (fn FourParamFn) Call(c *program.Caller, arg1, arg2, arg3, arg4 int64) (*program.Val, error) {
	return fn(c, arg1, arg2, arg3, arg4)
}

type FiveParamFn func(*program.Caller, int64, int64, int64, int64, int64) (*program.Val, error)

func (fn FiveParamFn) Call(c *program.Caller, arg1, arg2, arg3, arg4, arg5 int64) (*program.Val, error) {
	return fn(c, arg1, arg2, arg3, arg4, arg5)
}

type SixParamFn func(*program.Caller, int64, int64, int64, int64, int64, int64) (*program.Val, error)

func (fn SixParamFn) Call(c *program.Caller, arg1, arg2, arg3, arg4, arg5, arg6 int64) (*program.Val, error) {
	return fn(c, arg1, arg2, arg3, arg4, arg5, arg6)
}

// TODO: doc example of custom import function
// type CustomFn func(*program.Caller, int32, int64) (*program.Val, error)

// func (fn CustomFn) Call(c *program.Caller, arg1 int32, arg2 int64) (*program.Val, error) {
// 	return fn(c, arg1, arg2)
// }

func NewImportFn[F any](src F) func(caller *program.Caller, wargs ...wasmtime.Val) (*program.Val, error) {
	importFn := ImportFn[F]{fn: src}
	fn := func(c *program.Caller, wargs ...wasmtime.Val) (*program.Val, error) {
		args := make([]int64, 0, len(wargs))
		for _, arg := range wargs {
			args = append(args, arg.I64())
		}
		return importFn.Invoke(c, args...)
	}
	return fn
}

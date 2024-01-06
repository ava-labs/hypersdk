// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

// Func is a wrapper around a wasmtime.Func
type Func struct {
	inner *wasmtime.Func
	store wasmtime.Storelike
}

// NewFunc creates a new func wrapper.
func NewFunc(inner *wasmtime.Func, store wasmtime.Storelike) *Func {
	return &Func{
		inner: inner,
		store: store,
	}
}

func (f *Func) Call(params ...SmartPtr) ([]int64, error) {
	fnParams := f.Type().Params()
	if len(params) != len(fnParams) {
		return nil, fmt.Errorf("%w for function: %d expected: %d", ErrInvalidParamCount, len(params), len(fnParams))
	}

	// convert the args to the expected wasm types
	callParams, err := mapFunctionParams(params, fnParams)
	if err != nil {
		return nil, err
	}

	result, err := f.inner.Call(f.store, callParams...)
	if err != nil {
		return nil, HandleTrapError(err)
	}
	switch v := result.(type) {
	case int32:
		value := int64(result.(int32))
		return []int64{value}, nil
	case int64:
		value := result.(int64)
		return []int64{value}, nil
	case nil:
		// the function had no return values
		return nil, nil
	default:
		return nil, fmt.Errorf("invalid result type: %T", v)
	}
}

func (f *Func) Type() *wasmtime.FuncType {
	return f.inner.Type(f.store)
}

// mapFunctionParams maps call input to the expected wasm function params.
func mapFunctionParams(input []SmartPtr, values []*wasmtime.ValType) ([]interface{}, error) {
	params := make([]interface{}, len(values))
	for i, v := range values {
		switch v.Kind() {
		case wasmtime.KindI32:
			// ensure this value is within the range of an int32
			if !EnsureIntToInt32(int(input[i])) {
				return nil, fmt.Errorf("%w: %d", ErrOverflow, input[i])
			}
			params[i] = int32(input[i])
		case wasmtime.KindI64:
			params[i] = int64(input[i])
		default:
			return nil, fmt.Errorf("%w: %v", ErrInvalidParamType, v.Kind())
		}
	}

	return params, nil
}

// FuncName returns the name of the function which is support by the SDK.
func FuncName(name string) string {
	switch name {
	case AllocFnName, DeallocFnName, MemoryFnName:
		return name
	default:
		// the SDK will append the guest suffix to the function name
		return name + GuestSuffix
	}
}

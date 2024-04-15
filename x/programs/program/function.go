// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/near/borsh-go"
)

// Func is a wrapper around a wasmtime.Func
type Func struct {
	inner *wasmtime.Func
	inst  Instance
}

// NewFunc creates a new func wrapper.
func NewFunc(inner *wasmtime.Func, inst Instance) *Func {
	return &Func{
		inner: inner,
		inst:  inst,
	}
}

func (f *Func) Call(context Context, params ...SmartPtr) ([]int64, error) {
	fnParams := f.Type().Params()[1:] // strip program_id
	if len(params) != len(fnParams) {
		return nil, fmt.Errorf("%w for function: %d expected: %d", ErrInvalidParamCount, len(params), len(fnParams))
	}

	// convert the args to the expected wasm types
	callParams, err := mapFunctionParams(params, fnParams)
	if err != nil {
		return nil, err
	}

	mem, err := f.inst.Memory()
	if err != nil {
		return nil, err
	}
	contextPtr, err := argumentToSmartPtr(context, mem)
	if err != nil {
		return nil, err
	}

	result, err := f.inner.Call(f.inst.GetStore(), append([]interface{}{int64(contextPtr)}, callParams...)...)
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

func argumentToSmartPtr(obj interface{}, memory *Memory) (SmartPtr, error) {
	bytes, err := borsh.Serialize(obj)
	if err != nil {
		return 0, err
	}

	return BytesToSmartPtr(bytes, memory)
}

func (f *Func) Type() *wasmtime.FuncType {
	return f.inner.Type(f.inst.GetStore())
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

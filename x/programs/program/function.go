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

func (f *Func) Call(args ...int64) ([]int64, error) {
	fnArgs := f.Type().Params()
	if len(args) != len(fnArgs) {
		return nil, fmt.Errorf("%w for function: %d expected: %d", ErrInvalidArgCount, len(args), len(fnArgs))
	}

	// convert the args to the expected wasm types
	convertedArgs, err := convertCallArgs(args, fnArgs)
	if err != nil {
		return nil, err
	}

	result, err := f.inner.Call(f.store, convertedArgs...)
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
		return nil, fmt.Errorf("invalid result type: %v", v)
	}
}

func (f *Func) Type() *wasmtime.FuncType {
	return f.inner.Type(f.store)
}

// All call args are passed as int64, this converts them to the expected
// wasm types as defined by the function type.
func convertCallArgs(callArgs []int64, values []*wasmtime.ValType) ([]interface{}, error) {
	args := make([]interface{}, len(values))
	for i, v := range values {
		switch v.Kind() {
		case wasmtime.KindI32:
			// ensure this value is within the range of an int32
			if !EnsureInt64ToInt32(callArgs[i]) {
				return nil, fmt.Errorf("integer conversion %w: %d", ErrOverflow, args[i])
			}
			args[i] = int32(callArgs[i])
		case wasmtime.KindI64:
			args[i] = callArgs[i]
		default:
			return nil, fmt.Errorf("%w: %v", ErrInvalidArgType, v.Kind())
		}
	}

	return args, nil
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

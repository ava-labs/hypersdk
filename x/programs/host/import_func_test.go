// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package host

import (
	"testing"

	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/x/programs/program"
)

func TestXxx(t *testing.T) {
	require.New(t)
	OneParamFn := func(
	*program.Caller,
	int64) (*program.Val, error) {
		return nil, nil
	}
	module, fnName, paramCount, importFn := NewOneParamImport("env", "test", OneParamFn)
	require.Equal(t, "env", module)
	require.Equal(t, "test", fnName)
	require.Equal(t, 1, paramCount)
	fn := func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		val, err := importFn(program.NewCaller(caller), args...)
		if err != nil {
			return nil, wasmtime.NewTrap(err.Error())
		}
		return []wasmtime.Val{val.Wasmtime()}, nil
	}
	fn()Ptr
	
}

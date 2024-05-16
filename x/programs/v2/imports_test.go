// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package v2

import (
	"testing"

	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/stretchr/testify/require"
)

// TestImportsLinking ensures that the linker created by [createLinker] correctly creates callable host functions
func TestImportsLinking(t *testing.T) {
	require := require.New(t)

	called := false

	imports := NewImports()
	imports.AddModule(&ImportModule{
		name: "env",
		funcs: map[string]HostFunction{
			"alert": FunctionNoOutput(func(_ *CallInfo, _ []byte) error {
				called = true
				return nil
			}),
		},
	})
	engine := wasmtime.NewEngine()
	linker, err := imports.createLinker(engine, &CallInfo{})
	require.NoError(err)

	wasm, err := wasmtime.Wat2Wasm(`
	(module
      (import "env" "alert" (func $alert (param i32) (param i32)))
      (memory 1) ;; 1 pages
	  (func $run
	   i32.const 0
       i32.const 0
       call $alert
      )
      (export "memory" (memory 0))
      (export "run" (func $run))
    )	
	`)
	require.NoError(err)

	module, err := wasmtime.NewModule(engine, wasm)
	require.NoError(err)

	store := wasmtime.NewStore(engine)

	inst, err := linker.Instantiate(store, module)
	require.NoError(err)
	require.NotNil(inst)

	_, err = inst.GetExport(store, "run").Func().Call(store)
	require.NoError(err)
	require.True(called)
}

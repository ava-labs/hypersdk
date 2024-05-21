// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"testing"

	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/stretchr/testify/require"
)

// TestImportsLinking ensures that the linker created by [createLinker] correctly creates callable host functions
func TestImportsLinking(t *testing.T) {
	require := require.New(t)

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

	called := false

	imports := NewImports()
	imports.AddModule(&ImportModule{
		Name: "env",
		HostFunctions: map[string]HostFunction{
			"alert": {Function: FunctionNoOutput(func(_ *CallInfo, _ []byte) error {
				called = true
				return nil
			})},
		},
	})
	engine := wasmtime.NewEngine()
	store := wasmtime.NewStore(engine)
	callInfo := &CallInfo{inst: &ProgramInstance{store: store}}
	linker, err := imports.createLinker(engine, callInfo)
	require.NoError(err)

	module, err := wasmtime.NewModule(engine, wasm)
	require.NoError(err)

	inst, err := linker.Instantiate(store, module)
	require.NoError(err)
	require.NotNil(inst)

	_, err = inst.GetExport(store, "run").Func().Call(store)
	require.NoError(err)
	require.True(called)
}

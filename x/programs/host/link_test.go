// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package host

import (
	"testing"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/stretchr/testify/require"
)

func TestLinkInstantiated(t *testing.T) {
	require := require.New(t)

	// no imports
	wasm, err := wasmtime.Wat2Wasm(`
	(module
	  (memory 1) ;; 1 pages
	  (export "memory" (memory 0))
	)
	`)
	require.NoError(err)
	link, store := newTestLink(t, NoSupportedImports)
	require.NotNil(link)
	mod, err := store.CompileModule(wasm)
	require.NoError(err)
	_, err = link.Instantiate(store.Inner(), mod)
	require.NoError(err)
	err = link.RegisterFuncWrap("foo", "bar", func() {})
	require.ErrorIs(err, ErrInstantiated)

}

func TestLinkInstasntiated(t *testing.T) {
	require := require.New(t)

	wasm, err := wasmtime.Wat2Wasm(`
	(module
      (import "env" "alert" (func $alert (param i32)))
    )	
	`)
	require.NoError(err)
	link, store := newTestLink(t, NoSupportedImports)
	require.NotNil(link)
	mod, err := store.CompileModule(wasm)
	require.NoError(err)
	require.Equal(len(mod.Imports()), 1)
	err = link.RegisterFuncWrap("env", "alert", func() {})
	require.NoError(err)
	_, err = link.Instantiate(store.Inner(), mod)
	require.NoError(err)

}

func TestLinkImport(t *testing.T) {
	require := require.New(t)

	wasm, err := wasmtime.Wat2Wasm(`
	(module
      (import "env" "alert" (func $alert (param i32)))
    )	
	`)
	require.NoError(err)

	tests := []struct {
		name,
		module,
		fnName string
		errMsg string
		fn interface{}
	}{
		{
			name: "happy path",
			module: "env",
			fnName: "alert",
			fn: func(int32) {},
		},
		{
			name: "missing module",
			module: "oops",
			fnName: "alert",
			fn: func(int32) {},
			errMsg: "failed to find import module: env",
		},
		{
			name: "missing module function",
			module: "env",
			fnName: "oops",
			fn: func(int32) {},
			errMsg: "`env::alert` has not been defined",
		},
		{
			name: "invalid module function signature",
			module: "env",
			fnName: "alert",
			fn: func() {},
			errMsg: "function types incompatible",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imports := NewImportsBuilder()
			imports.Register(tt.module, func() Import {
				return newTestImport(tt.module, tt.fnName, tt.fn)
			})
			supported := imports.Build()
			link, store := newTestLink(t, supported)
			mod, err := store.CompileModule(wasm)
			require.NoError(err)
			_, err = link.Instantiate(store.Inner(), mod)
			if tt.errMsg != "" {
				require.ErrorContains(err, tt.errMsg)
				return
			}
			require.NoError(err)
		})
	}

}

func newTestLink(t *testing.T, supported SupportedImports) (*Link, engine.Store) {
	require := require.New(t)
	cfg, err := engine.NewConfigBuilder().Build()
	require.NoError(err)
	store, err := engine.NewStore(cfg)
	require.NoError(err)
	meter, err := engine.NewMeter(store, engine.NoUnits)
	require.NoError(err)
		
	return NewLink(logging.NoLog{}, store.Engine(), supported, meter, cfg), *store
}

type testImport struct {
	module string
	fnName string
	fn interface{}
}

func newTestImport(module,fnName string, fn interface{}) *testImport {
	return &testImport{
		module: module,
		fnName: fnName,
		fn: fn,
	}
}

func (i *testImport) Name() string {
	return i.module
}

func (i *testImport) Register(link *Link) error {
	return link.RegisterFuncWrap(i.Name(), i.fnName, i.fn)
}

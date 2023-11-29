// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package host

import (
	"fmt"
	"testing"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/stretchr/testify/require"
)

func TestLinkMissingImport(t *testing.T) {
	require := require.New(t)

	wasm, err := wasmtime.Wat2Wasm(`
	(module
      (import "env" "alert" (func $alert (param i32)))
    )	
	`)
	require.NoError(err)
	cfg, err := engine.NewConfigBuilder().Build()
	require.NoError(err)
	eng, err := engine.New(cfg)
	require.NoError(err)
	mod, err := eng.CompileModule(wasm)
	require.NoError(err)
	store, err := engine.NewStore(eng)
	require.NoError(err)
	link, err := newTestLink(cfg, store, NoSupportedImports)
	require.NoError(err)
	_, err = link.Instantiate(store, mod)
	require.ErrorIs(err, ErrMissingImportModule)
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
		fn     interface{}
	}{
		{
			name:   "happy path",
			module: "env",
			fnName: "alert",
			fn:     func(int32) {},
		},
		{
			name:   "missing module",
			module: "oops",
			fnName: "alert",
			fn:     func(int32) {},
			errMsg: "failed to find import module: env",
		},
		{
			name:   "missing module function",
			module: "env",
			fnName: "oops",
			fn:     func(int32) {},
			errMsg: "`env::alert` has not been defined",
		},
		{
			name:   "invalid module function signature",
			module: "env",
			fnName: "alert",
			fn:     func() {},
			errMsg: "function types incompatible",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imports := NewImportsBuilder()
			imports.Register(tt.module, func() Import {
				return newTestImport(tt.module, tt.fnName, []testFn{{fn: tt.fn, fnType: FnTypeCustom}})
			})
			cfg, err := engine.NewConfigBuilder().Build()
			require.NoError(err)
			eng, err := engine.New(cfg)
			require.NoError(err)
			mod, err := eng.CompileModule(wasm)
			require.NoError(err)
			store, err := engine.NewStore(eng)
			require.NoError(err)
			link, err := newTestLink(cfg, store, imports.Build())
			require.NoError(err)
			_, err = link.Instantiate(store, mod)
			if tt.errMsg != "" {
				require.ErrorContains(err, tt.errMsg) // can't use ErrorIs because the error message is not owned by us.
				return
			}
			require.NoError(err)
		})
	}

}

func newTestLink(cfg *engine.Config, store *engine.Store, supported SupportedImports) (*Link, error) {
	meter, err := engine.NewMeter(store, engine.NoUnits)
	if err != nil {
		return nil, err
	}
	return NewLink(logging.NoLog{}, store.Engine(), supported, meter, cfg), nil
}

type testFnType int

const (
	FnTypeInt64 testFnType = iota
	FnTypeCustom
)

type testFn struct {
	fn     interface{}
	fnType testFnType
}

type testImport struct {
	module string
	fnName string
	fns    []testFn
}

func newTestImport(module, fnName string, fns []testFn) *testImport {
	return &testImport{
		module: module,
		fnName: fnName,
		fns:    fns,
	}
}

func (i *testImport) Name() string {
	return i.module
}

func (i *testImport) Register(link *Link) error {
	for _, f := range i.fns {
		switch f.fnType {
		case FnTypeInt64:
			if err := link.RegisterInt64Fn(i.module, i.fnName, f); err != nil {
				return err
			}
		case FnTypeCustom:
			if err := link.RegisterFuncWrap(i.module, i.fnName, f.fn); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown fn type: %d", f.fnType)
		}
	}
	return nil
}

func BenchmarkInstantiate(b *testing.B) {
	require := require.New(b)
	imports := NewImportsBuilder()
	imports.Register("env", func() Import {
		return newTestImport("env", "alert", []testFn{{fn: func(int32) {}, fnType: FnTypeCustom}})
	})
	wasm, err := wasmtime.Wat2Wasm(`
	(module
	  (import "env" "alert" (func $alert (param i32)))
	)	
	`)
	require.NoError(err)
	cfg, err := engine.NewConfigBuilder().Build()
	require.NoError(err)
	eng, err := engine.New(cfg)
	require.NoError(err)
	mod, err := eng.CompileModule(wasm)
	require.NoError(err)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store, err := engine.NewStore(eng)
		require.NoError(err)
		link, err := newTestLink(cfg, store, imports.Build())
		require.NoError(err)
		_, err = link.Instantiate(store, mod)
		require.NoError(err)
	}
}

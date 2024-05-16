// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package host

import (
	"testing"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/program"
)

func TestLinkMissingImport(t *testing.T) {
	require := require.New(t)

	wasm, err := wasmtime.Wat2Wasm(`
	(module
      (import "env" "alert" (func $alert (param i32)))
    )	
	`)
	require.NoError(err)
	eng := engine.New(engine.NewConfig())
	mod, err := eng.CompileModule(wasm)
	require.NoError(err)
	store := engine.NewStore(eng, engine.NewStoreConfig())
	link, err := newTestLink(store, NoSupportedImports)
	require.NoError(err)
	_, err = link.Instantiate(store, mod, ImportFnCallback{}, &program.Context{})
	require.ErrorIs(err, ErrMissingImportModule)
}

func TestLinkImport(t *testing.T) {
	wasm, err := wasmtime.Wat2Wasm(`
	(module
      (import "env" "one" (func $one (param i64) (result i64)))
    )	
	`)
	require.NoError(t, err)

	tests := []struct {
		name,
		module,
		errMsg string
		fn interface{}
	}{
		{
			name:   "happy path",
			module: "env",
			fn:     func(int64) int64 { return 0 },
		},
		{
			name:   "missing module",
			module: "oops",
			fn:     func() {},
			errMsg: "failed to find import module: env",
		},
		{
			name:   "invalid module function signature",
			module: "env",
			fn:     func(int64) int32 { return 0 },
			errMsg: "function types incompatible",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			imports := NewImportsBuilder()
			imports.Register(tt.module, func() Import {
				return newTestImport(tt.module, tt.fn)
			})
			eng := engine.New(engine.NewConfig())
			mod, err := eng.CompileModule(wasm)
			require.NoError(err)
			store := engine.NewStore(eng, engine.NewStoreConfig())
			require.NoError(err)
			link, err := newTestLink(store, imports.Build())
			require.NoError(err)
			_, err = link.Instantiate(store, mod, ImportFnCallback{}, &program.Context{})
			if tt.errMsg != "" {
				require.ErrorContains(err, tt.errMsg) //nolint:forbidigo
				return
			}
			require.NoError(err)
		})
	}
}

// go test -v -benchmem -run=^$ -bench ^BenchmarkInstantiate$ github.com/ava-labs/hypersdk/x/programs/host -memprofile benchvset.mem -cpuprofile benchvset.cpu
func BenchmarkInstantiate(b *testing.B) {
	require := require.New(b)
	imports := NewImportsBuilder()
	imports.Register("env", func() Import {
		return newTestImport("env", nil)
	})
	wasm, err := wasmtime.Wat2Wasm(`
	(module
	  (import "env" "one" (func $one (param i64) (result i64)))
	)	
	`)
	require.NoError(err)
	eng := engine.New(engine.NewConfig())
	require.NoError(err)
	mod, err := eng.CompileModule(wasm)
	require.NoError(err)
	b.Run("benchmark_instantiate_instance", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			store := engine.NewStore(eng, engine.NewStoreConfig())
			link, err := newTestLink(store, imports.Build())
			require.NoError(err)
			_, err = link.Instantiate(store, mod, ImportFnCallback{}, &program.Context{})
			require.NoError(err)
		}
	})
}

func newTestLink(store *engine.Store, supported SupportedImports) (*Link, error) {
	meter, err := engine.NewMeter(store)
	if err != nil {
		return nil, err
	}
	return NewLink(logging.NoLog{}, store.GetEngine(), supported, meter, false), nil
}

type testImport struct {
	module string
	fn     interface{}
}

func newTestImport(module string, fn interface{}) *testImport {
	return &testImport{
		module: module,
		fn:     fn,
	}
}

func (i *testImport) Name() string {
	return i.module
}

func (i *testImport) Register(link *Link, _ *program.Context) error {
	if i.fn != nil {
		return link.RegisterImportFn(i.module, "one", i.fn)
	}
	return link.RegisterImportFn(i.module, "one", testOneParamFnWrap)
}

func testOneParamFnWrap(*wasmtime.Caller, int64) int64 {
	return 0
}

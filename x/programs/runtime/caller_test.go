// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"fmt"
	"testing"

	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/stretchr/testify/require"
)

func TestCaller(t *testing.T) {
	require := require.New(t)

	// export directly calls import so no units are consumed directly
	wasm, err := wasmtime.Wat2Wasm(`
	(module $test
	  (type (;0;) (func (param i64) (result i64)))
	  (import "test_import" "call" (func (;0;) (type 0)))
	  (export "get_guest" (func 0))
	) 
	`)

	// register import module
	supported := NewSupportedImports()
	supported.Register("test_import", func() Import {
		return newTestImport()
	})
	require.NoError(err)

	tests := []struct {
		name            string
		wasm            []byte
		maxUnits        uint64
		expectedBalance uint64
	}{
		{
			name:            "adequate balance context switch metered",
			wasm:            wasm,
			maxUnits:        2000,
			expectedBalance: defaultContextSwitchUnits,
		},
		{
			name:            "invalid balance context switch metered",
			wasm:            wasm,
			maxUnits:        900,
			expectedBalance: 0,
		},
		{
			name:            "invalid balance context switch metered",
			wasm:            wasm,
			maxUnits:        900,
			expectedBalance: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cfg, err := NewConfigBuilder().Build()
			require.NoError(err)

			rt := New(log, cfg, supported.Imports())
			err = rt.Initialize(ctx, wasm, tt.maxUnits)
			require.NoError(err)
			resp, err = rt.Call(ctx, "get", uint64(0))
			require.NoError(err)
			require.Equal(tt.expected, resp[0])
		})
	}

}

// test import
type testImport struct {
	imports    SupportedImports
	meter      Meter
	registered bool
}

// New returns a new program invoke host module which can perform program to program calls.
func newTestImport() *testImport {
	return &testImport{}
}

func (i *testImport) Name() string {
	return "test_import"
}

func (i *testImport) Register(link *Link, meter Meter, imports SupportedImports) error {
	if i.registered {
		return fmt.Errorf("import module already registered: %q", i.Name())
	}
	i.imports = imports
	i.meter = meter

	if err := link.RegisterFn(i.Name(), "call", i.testImportFn); err != nil {
		return err
	}

	return nil
}

// callProgramFn makes a call to an entry function of a program in the context of another program's ID.
func (i *testImport) testImportFn(
	caller *wasmtime.Caller,
	data int64,
) int64 {
	return int64(0)
}

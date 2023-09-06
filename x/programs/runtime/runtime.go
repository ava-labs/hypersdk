// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/x/programs/utils"
)

// New returns a new runtime instance. All runtimes are expected to run Stop
// when no longer in use.
func New(log logging.Logger, meter Meter, storage Storage) *runtime {
	return &runtime{
		log:      log,
		meter:    meter,
		storage:  storage,
		exported: make(map[string]api.Function),
	}
}

type runtime struct {
	cancelFn context.CancelFunc
	engine   wazero.Runtime
	client   *client
	meter    Meter
	storage  Storage
	exported map[string]api.Function
	db       chain.Database

	closed bool

	log logging.Logger
}

func (r *runtime) Initialize(ctx context.Context, programBytes []byte, functions []string) error {
	ctx, r.cancelFn = context.WithCancel(ctx)

	r.engine = wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())

	// register host modules
	mapMod := NewMapModule(r.log, r.meter)
	err := mapMod.Instantiate(ctx, r.engine)
	if err != nil {
		return fmt.Errorf("failed to create map host module: %w", err)
	}

	// enable program to program calls
	invokeMod := NewInvokeModule(r.log, r.db, r.meter, r.storage)
	err = invokeMod.Instantiate(ctx, r.engine)
	if err != nil {
		return fmt.Errorf("failed to create delegate host module: %w", err)
	}

	// TODO: remove/minimize preview1
	// Instantiate WASI, which implements system I/O such as console output.
	wasi_snapshot_preview1.MustInstantiate(ctx, r.engine)

	compiledModule, err := r.engine.CompileModule(ctx, programBytes)
	if err != nil {
		return fmt.Errorf("failed to compile wasm module: %w", err)
	}

	// TODO: breakout config?
	config := wazero.NewModuleConfig().
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		WithMeter(r.meter)

	mod, err := r.engine.InstantiateModule(ctx, compiledModule, config)
	if err != nil {
		return fmt.Errorf("failed to instantiate wasm module: %w", err)
	}

	// initialize exported functions
	exported := make(map[string]api.Function)

	// default functions
	exported[allocFnName] = mod.ExportedFunction(allocFnName)
	exported[deallocFnName] = mod.ExportedFunction(deallocFnName)

	for _, name := range functions {
		m := mod.ExportedFunction(utils.GetGuestFnName(name))
		if m == nil {
			return fmt.Errorf("failed to find exported function: %s", name)
		}
		exported[name] = mod.ExportedFunction(utils.GetGuestFnName(name))
	}

	r.client = newClient(r.log, mod, exported)

	return nil
}

func (r *runtime) Call(ctx context.Context, name string, params ...uint64) ([]uint64, error) {
	if r.closed {
		return nil, fmt.Errorf("failed to call: %s: runtime closed", name)
	}

	result, err := r.client.call(ctx, name, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to call %s: %w", name, err)
	}

	return result, nil
}

func (r *runtime) GetGuestBuffer(offset uint32, length uint32) ([]byte, bool) {
	return r.client.readMemory(offset, length)
}

func (r *runtime) WriteGuestBuffer(ctx context.Context, buf []byte) (uint64, error) {
	// allocate to guest and return offset
	offset, err := r.client.alloc(ctx, uint64(len(buf)))
	if err != nil {
		return 0, err
	}

	err = r.client.writeMemory(uint32(offset), buf)
	if err != nil {
		return 0, err
	}

	return offset, nil
}

func (r *runtime) Stop(ctx context.Context) {
	if r.closed {
		return
	}
	defer func() {
		r.client.Close(ctx)
		r.cancelFn()
	}()
	r.closed = true

	if err := r.engine.Close(ctx); err != nil {
		r.log.Error("failed to close wasm runtime",
			zap.Error(err),
		)
	}
}

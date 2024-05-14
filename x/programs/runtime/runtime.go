// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/program"
)

var _ Runtime = &WasmRuntime{}

// New returns a new wasm runtime.
func New(
	log logging.Logger,
	engine *engine.Engine,
	imports host.SupportedImports,
	cfg *Config,
) Runtime {
	return &WasmRuntime{
		log:     log,
		engine:  engine,
		imports: imports,
		cfg:     cfg,
	}
}

type WasmRuntime struct {
	engine  *engine.Engine
	meter   *engine.Meter
	inst    program.Instance
	imports host.SupportedImports
	cfg     *Config

	once     sync.Once
	cancelFn context.CancelFunc

	log logging.Logger
}

func (r *WasmRuntime) Initialize(ctx context.Context, callContext *program.Context, programBytes []byte, maxUnits uint64) (err error) {
	ctx, r.cancelFn = context.WithCancel(ctx)
	go func(ctx context.Context) {
		<-ctx.Done()
		// send immediate interrupt to engine
		r.Stop()
	}(ctx)

	// create store
	cfg := engine.NewStoreConfig()
	cfg.SetLimitMaxMemory(r.cfg.LimitMaxMemory)
	store := engine.NewStore(r.engine, cfg)

	// enable wasi logging support only in debug mode
	if r.cfg.EnableDebugMode {
		wasiConfig := wasmtime.NewWasiConfig()
		wasiConfig.InheritStderr()
		wasiConfig.InheritStdout()
		store.SetWasi(wasiConfig)
	}

	// add metered units to the store
	err = store.AddUnits(maxUnits)
	if err != nil {
		return err
	}

	// setup metering
	r.meter, err = engine.NewMeter(store)
	if err != nil {
		return err
	}

	// create module
	mod, err := engine.NewModule(r.engine, programBytes, r.cfg.CompileStrategy)
	if err != nil {
		return err
	}

	// create linker
	link := host.NewLink(r.log, store.GetEngine(), r.imports, r.meter, r.cfg.EnableDebugMode)

	// instantiate the module with all of the imports defined by the linker
	inst, err := link.Instantiate(store, mod, r.cfg.ImportFnCallback, callContext)
	if err != nil {
		return err
	}

	// set the instance
	r.inst = NewInstance(store, inst)

	return nil
}

func (r *WasmRuntime) Call(_ context.Context, name string, context *program.Context, params ...uint32) ([]byte, error) {
	fn, err := r.inst.GetFunc(name)
	if err != nil {
		return nil, err
	}

	return fn.Call(context, params...)
}

func (r *WasmRuntime) Memory() (*program.Memory, error) {
	return r.inst.Memory()
}

func (r *WasmRuntime) Meter() *engine.Meter {
	return r.meter
}

func (r *WasmRuntime) Stop() {
	r.once.Do(func() {
		r.log.Debug("shutting down runtime engine...")
		// send immediate interrupt to engine and all children stores.
		r.engine.Stop()
		r.cancelFn()
	})
}

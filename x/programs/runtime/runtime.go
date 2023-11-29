// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"sync"

	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/program"
)

var _ Runtime = &WasmRuntime{}

// New returns a new wasm runtime.
func New(log logging.Logger, cfg *engine.Config, imports host.SupportedImports) Runtime {
	return &WasmRuntime{
		imports: imports,
		log:     log,
		cfg:     cfg,
	}
}

type WasmRuntime struct {
	cfg    *engine.Config
	inst   program.Instance
	meter  engine.Meter
	engine *wasmtime.Engine

	once     sync.Once
	cancelFn context.CancelFunc

	imports host.SupportedImports

	log logging.Logger
}

func (r *WasmRuntime) Initialize(ctx context.Context, programBytes []byte, maxUnits uint64) (err error) {
	ctx, r.cancelFn = context.WithCancel(ctx)
	go func(ctx context.Context) {
		<-ctx.Done()
		// send immediate interrupt to engine
		r.Stop()
	}(ctx)

	eng, err := engine.New(r.cfg)
	if err != nil {
		return err
	}

	store, err := engine.NewStore(eng)
	if err != nil {
		return err
	}

	// set initial epoch deadline
	store.SetEpochDeadline(1)

	mod, err := eng.CompileModule(programBytes)
	if err != nil {
		return err
	}

	// setup metering
	r.meter, err = engine.NewMeter(store, maxUnits)
	if err != nil {
		return err
	}

	link := host.NewLink(r.log, store.Engine(), r.imports, r.meter, r.cfg)

	// enable wasi logging support only in testing/debug mode
	if r.cfg.DebugMode {
		wasiConfig := wasmtime.NewWasiConfig()
		wasiConfig.InheritStderr()
		wasiConfig.InheritStdout()
		store.SetWasi(wasiConfig)
		err = link.Wasi()
		if err != nil {
			return err
		}
	}

	// instantiate the module with all of the imports defined by the linker
	inst, err := link.Instantiate(store, mod)
	if err != nil {
		return err
	}

	r.inst = NewInstance(store, inst)
	r.engine = store.Engine()

	return nil
}

func (r *WasmRuntime) Call(_ context.Context, name string, args ...int64) ([]int64, error) {
	fn, err := r.inst.GetFunc(name)
	if err != nil {
		return nil, err
	}

	return fn.Call(args...)
}

func (r *WasmRuntime) Memory() (*program.Memory, error) {
	return r.inst.Memory()
}

func (r *WasmRuntime) Meter() engine.Meter {
	return r.meter
}

func (r *WasmRuntime) Stop() {
	r.once.Do(func() {
		r.log.Debug("shutting down runtime engine...")
		// send immediate interrupt to engine
		r.engine.IncrementEpoch()
		r.cancelFn()
	})
}

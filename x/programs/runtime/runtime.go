// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/utils/logging"

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
	inst    program.Instance
	meter   program.Meter
	imports host.SupportedImports
	cfg     *Config

	once     sync.Once
	cancelFn context.CancelFunc

	log logging.Logger
}

func (r *WasmRuntime) Initialize(
	ctx context.Context,
	programBytes []byte,
	maxUnits uint64,
) error {
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

	// set initial epoch deadline
	store.SetEpochDeadline(1)

	// compile the module
	mod, err := engine.NewModule(r.engine, programBytes, r.cfg.CompileStrategy)
	if err != nil {
		return err
	}

	// setup metering
	r.meter, err = program.NewMeter(store, maxUnits)
	if err != nil {
		return err
	}

	// create linker
	link := host.NewLink(r.log, store.Engine(), r.imports, r.meter, r.cfg.EnableDebugMode)

	// instantiate the module with all of the imports defined by the linker
	inst, err := link.Instantiate(store, mod, nil)
	if err != nil {
		return err
	}

	// set the instance
	r.inst = NewInstance(store, inst)

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

func (r *WasmRuntime) Meter() program.Meter {
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

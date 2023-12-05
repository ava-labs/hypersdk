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
func New(log logging.Logger, cfg *Config, imports host.SupportedImports) Runtime {
	return &WasmRuntime{
		imports: imports,
		log:     log,
		cfg:     cfg,
	}
}

type WasmRuntime struct {
	cfg     *Config
	engine  *engine.Engine
	inst    program.Instance
	meter   program.Meter
	imports host.SupportedImports

	once     sync.Once
	cancelFn context.CancelFunc

	log logging.Logger
}

func (r *WasmRuntime) Initialize(ctx context.Context, programBytes []byte, maxUnits uint64) (err error) {
	ctx, r.cancelFn = context.WithCancel(ctx)
	go func(ctx context.Context) {
		<-ctx.Done()
		// send immediate interrupt to engine
		r.Stop()
	}(ctx)

	ecfg, err := r.cfg.EngineCfg()
	if err != nil {
		return err
	}

	r.engine = engine.New(ecfg)
	store := engine.NewStore(r.engine, r.cfg.StoreCfg())
	if err != nil {
		return err
	}

	// set initial epoch deadline
	store.SetEpochDeadline(1)

	// compile the module
	mod, err := r.engine.CompileModule(programBytes)
	if err != nil {
		return err
	}

	// setup metering
	r.meter, err = program.NewMeter(store, maxUnits)
	if err != nil {
		return err
	}

	// create linker
	link := host.NewLink(r.log, store.Engine(), r.imports, r.meter, r.cfg.DebugMode)

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

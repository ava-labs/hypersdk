// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/utils"
)

const (
	allocFnName   = "alloc"
	deallocFnName = "dealloc"
)
// TODO: remove database.Database to make this runtime more generic
func New(log logging.Logger, meter Meter, db database.Database) *runtime {
	return &runtime{
		log:   log,
		meter: meter,
		db:    db,
	}
}

type runtime struct {
	cancelFn context.CancelFunc
	engine   wazero.Runtime
	mod      api.Module
	meter    Meter
	// functions exported by this runtime
	mu state.Mutable

	closed bool
	log    logging.Logger
	// db for persistent storage
	db database.Database
}

func (r *runtime) initProgramStorage(programBytes []byte) (int64, error) {
	count, err := GetProgramCount(r.db)
	if err != nil {
		r.log.Error("failed to get program counter", zap.Error(err))
		return 0, err
	}
	// increment
	err = IncrementProgramCount(r.db)
	if err != nil {
		r.log.Error("failed to increment program counter", zap.Error(err))
		return 0, err
	}
	// store program bytes
	err = SetProgram(r.db, count, programBytes)
	if err != nil {
		r.log.Error("failed to set program", zap.Error(err))
		return 0, err
	}
	return int64(count), nil
}

func (r *runtime) Create(ctx context.Context, programBytes []byte) (uint64, error) {
	err := r.Initialize(ctx, programBytes)
	if err != nil {
		return 0, err
	}
	// get programId
	programID, err := r.initProgramStorage(programBytes)
	if err != nil {
		return 0, err
	}
	// call initialize if it exists
	result, err := r.Call(ctx, "init", uint64(programID))
	if err != nil {
		if !errors.Is(err, ErrMissingExportedFunction) {
			return 0, err
		}
	} else {
		// check boolean result from init
		if result[0] == 0 {
			return 0, fmt.Errorf("failed to initialize program")
		}
	}
	return uint64(programID), nil
}

func (r *runtime) Initialize(ctx context.Context, programBytes []byte) error {
	ctx, r.cancelFn = context.WithCancel(ctx)

	r.engine = wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())

	// register host modules
	mapMod := NewMapModule(r.log, r.meter, r.db)
	err := mapMod.Instantiate(ctx, r.engine)
	if err != nil {
		return fmt.Errorf("failed to create map host module: %w", err)
	}

	// enable program to program calls
	invokeMod := NewInvokeModule(r.log, r.mu, r.meter, r.db)
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

	r.mod, err = r.engine.InstantiateModule(ctx, compiledModule, config)
	if err != nil {
		return fmt.Errorf("failed to instantiate wasm module: %w", err)
	}

	r.log.Debug("Instantiated module")

	return nil
}

func (r *runtime) GetCurrentGas(ctx context.Context) uint64 {
	return r.meter.GetBalance(ctx)
}

func (r *runtime) Call(ctx context.Context, name string, params ...uint64) ([]uint64, error) {
	if r.closed {
		return nil, fmt.Errorf("failed to call: %s: runtime closed", name)
	}

	var api api.Function

	if name == allocFnName || name == deallocFnName {
		api = r.mod.ExportedFunction(name)
	} else {
		api = r.mod.ExportedFunction(utils.GetGuestFnName(name))
	}

	if api == nil {
		return nil, fmt.Errorf("%w: %s", ErrMissingExportedFunction, name)
	}

	result, err := api.Call(ctx, params...)
	if err != nil {
		return nil, fmt.Errorf("failed to call %s: %w", name, err)
	}

	return result, nil
}

func (r *runtime) GetGuestBuffer(offset uint32, length uint32) ([]byte, bool) {
	// TODO: add fee
	// r.meter.AddCost()

	return r.mod.Memory().Read(offset, length)
}

// TODO: ensure deallocate on Stop.
func (r *runtime) WriteGuestBuffer(ctx context.Context, buf []byte) (uint64, error) {
	// TODO: add fee
	// r.meter.AddCost()

	// allocate to guest and return offset
	result, err := r.Call(ctx, allocFnName, uint64(len(buf)))
	if err != nil {
		return 0, err
	}

	offset := result[0]
	ok := r.mod.Memory().Write(uint32(offset), buf)
	if !ok {
		return 0, fmt.Errorf("failed to write at offset: %d size: %d", offset, r.mod.Memory().Size())
	}

	return offset, nil
}

func (r *runtime) Stop(ctx context.Context) error {
	defer r.cancelFn()
	if r.closed {
		return nil
	}
	r.closed = true

	if err := r.engine.Close(ctx); err != nil {
		return fmt.Errorf("failed to close wasm runtime: %w", err)
	}
	if err := r.mod.Close(ctx); err != nil {
		return fmt.Errorf("failed to close wasm api module: %w", err)
	}
	return nil
}

func (r *runtime) GetUserData() (map[string]int, error) {
	functionDef := r.mod.ExportedFunctionDefinitions()
	// loop through
	keys := make(map[string]int)
	for k := range functionDef {
		// TODO: don't hardcode init
		if k != allocFnName && k != deallocFnName && strings.Contains(k, utils.FunctionSuffix) && k != "init_guest" {
			replacement := ""
			funcName := strings.ReplaceAll(k, utils.FunctionSuffix, replacement)
			// get exported function
			api := r.mod.ExportedFunction(k)
			// we subtract one for the program id param
			keys[funcName] = len(api.Definition().ParamTypes()) - 1
			// defensive check
			if keys[funcName] < 0 {
				return nil, fmt.Errorf("failed to get user data")
			}
		}
	}
	return keys, nil
}

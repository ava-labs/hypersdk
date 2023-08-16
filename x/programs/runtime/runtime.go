package runtime

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/x/programs/meter"
	"github.com/ava-labs/hypersdk/x/programs/utils"
)

const (
	allocFnName   = "alloc"
	deallocFnName = "dealloc"
)

func New(log logging.Logger, db chain.Database, meter meter.Meter, programPrefix byte) *runtime {
	return &runtime{
		log:           log,
		db:            db,
		programPrefix: programPrefix,
		exported:      make(map[string]api.Function),
	}
}

type runtime struct {
	engine        wazero.Runtime
	mod           api.Module
	meter         meter.Meter
	programPrefix byte
	// functions exported by this runtime
	exported map[string]api.Function
	db       chain.Database

	lock   sync.Mutex
	closed bool

	log logging.Logger
}

func (r *runtime) Initialize(ctx context.Context, programBytes []byte, functions []string) error {
	r.engine = wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter())

	mapMod := NewMapModule(r.log, r.meter)
	err := mapMod.Instantiate(ctx, r.engine)
	if err != nil {
		return fmt.Errorf("failed to create host module: %w", err)
	}

	// enable program to program call delegation
	delegateMod := NewDelegateModule(r.log, r.db, r.meter, r.programPrefix)
	err = delegateMod.Instantiate(ctx, r.engine)
	if err != nil {
		return fmt.Errorf("failed to create delegate host module: %w", err)
	}

	// TODO: remove preview1
	// Instantiate WASI, which implements system I/O such as console output.
	wasi_snapshot_preview1.MustInstantiate(ctx, r.engine)

	compiledModule, err := r.engine.CompileModule(ctx, programBytes)
	if err != nil {
		return fmt.Errorf("compiling wasm module: %w", err)
	}

	// TODO: breakout config?
	config := wazero.NewModuleConfig().
		WithStdout(os.Stdout).
		WithStderr(os.Stderr).
		WithMeter(r.meter)

	r.mod, err = r.engine.InstantiateModule(ctx, compiledModule, config)
	if err != nil {
		return fmt.Errorf("failed to instantiate wasm module: %v", err)
	}
	r.log.Debug("Instantiated module")

	// TODO: cleanup
	for _, name := range functions {
		switch name {
		case allocFnName, "dealloc":
			r.exported[name] = r.mod.ExportedFunction(name)
		default:
			r.exported[name] = r.mod.ExportedFunction(utils.GetGuestFnName(name))
		}
	}

	go r.startMeter(ctx)

	return nil
}

func (r *runtime) Call(ctx context.Context, name string, params ...uint64) ([]uint64, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.closed {
		return nil, fmt.Errorf("failed to call: %s: runtime closed", name)
	}

	api, ok := r.exported[name]
	if !ok {
		return nil, fmt.Errorf("failed to find exported function: %s", name)
	}

	return api.Call(ctx, params...)
}

// GetGuestBuffer returns a buffer from the guest at [offset] with length [length]. Returns
// false if out of range.
func (r *runtime) GetGuestBuffer(offset uint32, length uint32) ([]byte, bool) {
	// TODO: add fee
	// r.meter.AddCost()

	return r.mod.Memory().Read(offset, length)
}

//	WriteGuestBuffer allocates buf to the heap on the guest and returns the offset.
//
// TODO: add all offset and len to a stack and ensure deallocate on Stop.
func (r *runtime) WriteGuestBuffer(ctx context.Context, buf []byte) (uint64, error) {
	// TODO: add fee
	// r.meter.AddCost()

	// allocate to heap on guest and return offset
	result, err := r.Call(ctx, allocFnName, uint64(len(buf)))
	if err != nil {
		return 0, err
	}

	offset := result[0]
	ok := r.mod.Memory().Write(uint32(offset), buf[:])
	if !ok {
		return 0, fmt.Errorf("failed to write guest heap at offset: %d size: %d", offset, r.mod.Memory().Size())
	}

	return offset, nil
}

func (r *runtime) Stop(ctx context.Context) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true

	if err := r.engine.Close(ctx); err != nil {
		return fmt.Errorf("failed to close wasm runtime: %w", err)
	}
	return nil
}

func (r *runtime) startMeter(ctx context.Context) {
	defer func() {
		err := r.Stop(ctx)
		r.log.Error("failed to shutdown runtime:",
			zap.Error(err),
		)
	}()

	err := r.meter.Run(ctx)
	if err != nil {
		r.log.Error("meter failed",
			zap.Error(err),
		)
	}
}

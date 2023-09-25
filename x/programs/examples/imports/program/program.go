// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v12"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

const Name = "program"

type Import struct {
	db         state.Immutable
	log        logging.Logger
	imports    runtime.Imports
	meter      runtime.Meter
	registered bool
}

// New returns a new program invoke host module which can perform program to program calls.
func New(log logging.Logger, db state.Immutable) *Import {
	return &Import{
		db:  db,
		log: log,
	}
}

func (i *Import) Name() string {
	return Name
}

func (i *Import) Register(link runtime.Link, meter runtime.Meter, imports runtime.Imports) error {
	if i.registered {
		return fmt.Errorf("import module already registered")
	}
	if err := link.FuncWrap(Name, "invoke_program", i.invokeProgramFn); err != nil {
		return err
	}
	i.registered = true
	i.imports = imports
	i.meter = meter

	return nil
}

// invokeProgramFn makes a call to an entry function of a program in the context of another program's ID.
func (i *Import) invokeProgramFn(
	caller *wasmtime.Caller,
	callerIDPtr int64,
	programIDPtr int64,
	maxUnits int64,
	functionPtr,
	functionLen,
	argsPtr,
	argsLen int32,
) int64 {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	memory := runtime.NewMemory(runtime.NewExportClient(caller))

	// get the entry function for invoke to call.
	functionBytes, err := memory.Range(uint64(functionPtr), uint64(functionLen))
	if err != nil {
		i.log.Error("failed to read function name from memory",
			zap.Error(err),
		)
		return -1
	}

	programIDBytes, err := memory.Range(uint64(programIDPtr), uint64(ids.IDLen))
	if err != nil {
		i.log.Error("failed to read id from memory",
			zap.Error(err),
		)
		return -1
	}

	// get the program bytes from storage
	programWasmBytes, err := getProgramWasmBytes(i.log, i.db, programIDBytes)
	if err != nil {
		i.log.Error("failed to get program bytes from storage",
			zap.Error(err),
		)
		return -1
	}

	// initialize a new runtime config with zero balance
	cfg, err := runtime.NewConfigBuilder(runtime.NoUnits).
		WithBulkMemory(true).
		WithLimitMaxMemory(17 * runtime.MemoryPageSize). // 17 pages
		Build()
	if err != nil {
		i.log.Error("failed to create runtime config",
			zap.Error(err),
		)
		return -1
	}

	// create a new runtime for the program to be invoked
	rt := runtime.New(i.log, cfg, i.imports)
	err = rt.Initialize(context.Background(), programWasmBytes)
	if err != nil {
		i.log.Error("failed to initialize runtime",
			zap.Error(err),
		)
		return -1
	}

	// transfer the units from the caller to the new runtime before any calls are made.
	_, err = i.meter.TransferUnits(rt.Meter(), uint64(maxUnits))
	if err != nil {
		i.log.Error("failed to transfer units",
			zap.Uint64("balance", i.meter.GetBalance()),
			zap.Int64("required", maxUnits),
			zap.Error(err),
		)
		return -1
	}

	// write the program id to the new runtime memory
	ptr, err := runtime.WriteBytes(rt.Memory(), programIDBytes)
	if err != nil {
		i.log.Error("failed to write program id to memory",
			zap.Error(err),
		)
		return -1
	}

	argsBytes, err := memory.Range(uint64(argsPtr), uint64(argsLen))
	if err != nil {
		i.log.Error("failed to read program args name from memory",
			zap.Error(err),
		)
		return -1
	}

	// sync args to new runtime and return arguments to the invoke call
	params, err := getCallArgs(ctx, rt, argsBytes, ptr)
	if err != nil {
		i.log.Error("failed to unmarshal call arguments",
			zap.Error(err),
		)
		return -1
	}

	for _, param := range params {
		i.log.Debug("param", zap.Uint64("param", param))
	}

	function := string(functionBytes)
	res, err := rt.Call(ctx, function, params...)
	if err != nil {
		i.log.Error("failed to call entry function",
			zap.Error(err),
		)
		return -1
	}

	// stop the runtime to prevent further execution
	rt.Stop()

	_, err = rt.Meter().TransferUnits(i.meter, rt.Meter().GetBalance())
	if err != nil {
		i.log.Error("failed to transfer remaining balance to caller",
			zap.Error(err),
		)
		return -1
	}

	return int64(res[0])
}

func getCallArgs(ctx context.Context, rt runtime.Runtime, buffer []byte, invokeProgramID uint64) ([]uint64, error) {
	// first arg contains id of program to call
	args := []uint64{invokeProgramID}
	p := codec.NewReader(buffer, len(buffer))
	i := 0
	for !p.Empty() {
		size := p.UnpackInt64(true)
		isInt := p.UnpackBool()
		if isInt {
			valueInt := p.UnpackUint64(true)
			args = append(args, valueInt)
			fmt.Printf("valueBytes: %d: %v\n", i, valueInt)
		} else {
			valueBytes := make([]byte, size)
			p.UnpackFixedBytes(int(size), &valueBytes)
			fmt.Printf("valueBytes: %d: %v\n", i, valueBytes)
			ptr, err := runtime.WriteBytes(rt.Memory(), valueBytes)
			if err != nil {
				return nil, err
			}
			args = append(args, ptr)
		}
		i++
	}
	if p.Err() != nil {
		return nil, fmt.Errorf("failed to unpack arguments: %w", p.Err())
	}
	return args, nil
}

func getProgramWasmBytes(log logging.Logger, db state.Immutable, idBytes []byte) ([]byte, error) {
	id, err := ids.ToID(idBytes)
	if err != nil {
		return nil, err
	}

	// get the program bytes from storage
	bytes, exists, err := storage.GetProgram(context.Background(), db, id)
	if !exists {
		log.Debug("key does not exist", zap.String("id", id.String()))
	}
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

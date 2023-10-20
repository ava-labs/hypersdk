// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v13"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

const Name = "program"

type Import struct {
	cfg        *runtime.Config
	db         state.Mutable
	log        logging.Logger
	imports    runtime.SupportedImports
	meter      runtime.Meter
	registered bool
}

// New returns a new program invoke host module which can perform program to program calls.
func New(log logging.Logger, db state.Mutable, cfg *runtime.Config) *Import {
	return &Import{
		cfg: cfg,
		db:  db,
		log: log,
	}
}

func (i *Import) Name() string {
	return Name
}

func (i *Import) Register(link runtime.Link, meter runtime.Meter, imports runtime.SupportedImports) error {
	if i.registered {
		return fmt.Errorf("import module already registered: %q", Name)
	}
	i.imports = imports
	i.meter = meter

	if err := link.FuncWrap(Name, "call_program", i.callProgramFn); err != nil {
		return err
	}

	return nil
}

// callProgramFn makes a call to an entry function of a program in the context of another program's ID.
func (i *Import) callProgramFn(
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

	// create a new runtime for the program to be invoked with a zero balance.
	rt := runtime.New(i.log, i.cfg, i.imports)
	err = rt.Initialize(context.Background(), programWasmBytes, runtime.NoUnits)
	if err != nil {
		i.log.Error("failed to initialize runtime",
			zap.Error(err),
		)
		return -1
	}

	// transfer the units from the caller to the new runtime before any calls are made.
	_, err = i.meter.TransferUnitsTo(rt.Meter(), uint64(maxUnits))
	if err != nil {
		i.log.Error("failed to transfer units",
			zap.Uint64("balance", i.meter.GetBalance()),
			zap.Int64("required", maxUnits),
			zap.Error(err),
		)
		return -1
	}

	// transfer remaining balance back to parent runtime
	defer func() {
		// stop the runtime to prevent further execution
		rt.Stop()

		_, err = rt.Meter().TransferUnitsTo(i.meter, rt.Meter().GetBalance())
		if err != nil {
			i.log.Error("failed to transfer remaining balance to caller",
				zap.Error(err),
			)
		}
	}()

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

	function := string(functionBytes)
	res, err := rt.Call(ctx, function, params...)
	if err != nil {
		i.log.Error("failed to call entry function",
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
		} else {
			valueBytes := make([]byte, size)
			p.UnpackFixedBytes(int(size), &valueBytes)
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

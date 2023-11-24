// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/program"
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

func (i *Import) Register(link program.Link, meter runtime.Meter, imports runtime.SupportedImports) error {
	if i.registered {
		return fmt.Errorf("import module already registered: %q", Name)
	}
	
	i.registered = true
	i.imports = imports
	i.meter = meter

	return link.RegisterFn(program.NewFiveParamImport(Name, "call_program", i.callProgramFn))
}

func (i *Import) callProgramFn(
	caller program.Caller,
	callerID,
	programID,
	maxUnits,
	function,
	args int64,
) (*program.Val, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	memory, err := caller.Memory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory: %w", err)
	}

	functionBytes, err := program.Int64ToBytes(memory,function)
	if err != nil {
		return nil, fmt.Errorf("failed to read function name from memory: %w", err)
	}

	programIDBytes, err := program.Int64ToBytes(memory,programID)
	if err != nil {
		return nil, fmt.Errorf("failed to read id from memory: %w", err)
	}

	// get the program bytes from storage
	programWasmBytes, err := getProgramWasmBytes(i.log, i.db, programIDBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to get program bytes from storage: %w", err)
	}

	// create a new runtime for the program to be invoked with a zero balance.
	rt := runtime.New(i.log, i.cfg, i.imports)
	err = rt.Initialize(ctx, programWasmBytes, runtime.NoUnits)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize child runtime: %w", err)
	}

	// transfer the units from the caller to the new runtime before any calls are made.
	_, err = i.meter.TransferUnitsTo(rt.Meter(), uint64(maxUnits))
	if err != nil {
		i.log.Error("failed to transfer units",
			zap.Uint64("balance", i.meter.GetBalance()),
			zap.Int64("required", maxUnits),
			zap.Error(err),
		)
		return nil, err
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
	ptr, err := program.WriteBytes(rt.Memory(), programIDBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to write program id to memory: %w", err)
	}

	argsBytes, err := program.Int64ToBytes(memory,args)
	if err != nil {
		return nil, fmt.Errorf("failed to read program args name from memory: %w", err)
	}

	// sync args to new runtime and return arguments to the invoke call
	params, err := getCallArgs(ctx, rt.Memory(), argsBytes, ptr)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal call arguments: %w", err)
	}

	resp, err := rt.Call(ctx, string(functionBytes), params...)
	if err != nil {
		return nil, fmt.Errorf("failed to call entry function: %w", err)
	}

	return program.ValI64(resp[0]), nil
}

func getCallArgs(ctx context.Context, memory *program.Memory, buffer []byte, invokeProgramID int64) ([]int64, error) {
	// first arg contains id of program to call
	args := []int64{invokeProgramID}
	p := codec.NewReader(buffer, len(buffer))
	i := 0
	for !p.Empty() {
		size := p.UnpackInt64(true)
		isInt := p.UnpackBool()
		if isInt {
			valueInt := p.UnpackInt64(true)
			args = append(args, valueInt)
		} else {
			valueBytes := make([]byte, size)
			p.UnpackFixedBytes(int(size), &valueBytes)
			ptr, err := program.WriteBytes(memory, valueBytes)
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

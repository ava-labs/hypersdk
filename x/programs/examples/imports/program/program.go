// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package imports

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

func (i *Import) Register(linker runtime.Link, meter runtime.Meter, imports runtime.Imports) error {
	if i.registered {
		return fmt.Errorf("import module already registered")
	}
	if err := linker.FuncWrap(Name, "invoke_program", i.invokeProgramFn); err != nil {
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
	programIDPtr,
	functionPtr,
	functionLen,
	argsPtr,
	argsLen uint32,
	maxUnits uint64,
) int64 {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	memory := runtime.NewMemory(runtime.NewExportClient(caller))

	programIDBytes, err := memory.Range(uint32(programIDPtr), uint32(ids.IDLen))
	if err != nil {
		i.log.Error("failed to read function name from memory",
			zap.Error(err),
		)
		return -1
	}

	programID, err := ids.ToID(programIDBytes)
	if err != nil {
		i.log.Error("failed to create programID",
			zap.Error(err),
		)
		return -1
	}

	// get the entry function for invoke to call.
	functionBytes, err := memory.Range(uint32(functionPtr), uint32(functionLen))
	if err != nil {
		i.log.Error("failed to read function name from memory",
			zap.Error(err),
		)
		return -1
	}

	programWasmBytes, exists, err := storage.GetProgram(i.db, programID)
	if !exists {
		i.log.Error("program does not exist")
	}
	if err != nil {
		i.log.Error("failed to get program from storage",
			zap.Error(err),
		)
		return -1
	}

	// spend the maximum number of units allowed for this call
	i.meter.Spend(maxUnits)

	cfg, err := runtime.NewConfigBuilder(maxUnits).Build()
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

	argsBytes, err := memory.Range(uint32(argsPtr), uint32(argsLen))
	if err != nil {
		i.log.Error("failed to read program args name from memory",
			zap.Error(err),
		)
		return -1
	}

	// sync args to new runtime and return arguments to the invoke call
	params, err := getCallArgs(ctx, rt, argsBytes, programIDPtr)
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

	// spend the remaining balance
	balance := rt.Meter().GetBalance()
	_, err = rt.Meter().Spend(balance)
	if err != nil {
		i.log.Error("failed to spend balance",
			zap.Error(err),
		)
		return -1
	}

	// return balance to parent runtime
	i.meter.AddUnits(balance)

	return int64(res[0])
}

func getCallArgs(ctx context.Context, rt runtime.Runtime, buffer []byte, invokeProgramID uint32) ([]interface{}, error) {
	// first arg contains id of program to call
	args := []interface{}{invokeProgramID}
	p := codec.NewReader(buffer, len(buffer))
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
	}
	if p.Err() != nil {
		return nil, fmt.Errorf("failed to unpack arguments: %w", p.Err())
	}
	return args, nil
}

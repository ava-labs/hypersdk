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
	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/program"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

const Name = "program"

type Import struct {
	cfg  *engine.Config
	db   state.Mutable
	log  logging.Logger
	link *host.Link
}

// New returns a new program invoke host module which can perform program to program calls.
func New(log logging.Logger, db state.Mutable, cfg *engine.Config) *Import {
	return &Import{
		cfg: cfg,
		db:  db,
		log: log,
	}
}

func (i *Import) Name() string {
	return Name
}

func (i *Import) Register(link host.Link) error {
	return link.RegisterFiveParamInt64Fn(Name, "call_program", i.callProgramFn)
}

func (i *Import) callProgramFn(
	caller *program.Caller,
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

	functionBytes, err := program.Int64ToBytes(memory, function)
	if err != nil {
		return nil, fmt.Errorf("failed to read function name from memory: %w", err)
	}

	programIDBytes, err := program.Int64ToBytes(memory, programID)
	if err != nil {
		return nil, fmt.Errorf("failed to read id from memory: %w", err)
	}

	// get the program bytes from storage
	programWasmBytes, err := getProgramWasmBytes(i.log, i.db, programIDBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to get program bytes from storage: %w", err)
	}

	// create a new runtime for the program to be invoked with a zero balance.
	rt := runtime.New(i.link.Log(), i.cfg, i.link.Imports())
	err = rt.Initialize(ctx, programWasmBytes, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize child runtime: %w", err)
	}

	// transfer the units from the caller to the new runtime before any calls are made.
	_, err = i.link.Meter().TransferUnitsTo(rt.Meter(), uint64(maxUnits))
	if err != nil {
		i.log.Error("failed to transfer units",
			zap.Uint64("balance", i.link.Meter().GetBalance()),
			zap.Int64("required", maxUnits),
			zap.Error(err),
		)
		return nil, err
	}

	// transfer remaining balance back to parent runtime
	defer func() {
		// stop the runtime to prevent further execution
		rt.Stop()

		_, err = rt.Meter().TransferUnitsTo(i.link.Meter(), rt.Meter().GetBalance())
		if err != nil {
			i.log.Error("failed to transfer remaining balance to caller",
				zap.Error(err),
			)
		}
	}()

	newMem, err := rt.Memory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory: %w", err)
	}

	// write the program id to the new runtime memory
	ptr, err := program.WriteBytes(newMem, programIDBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to write program id to memory: %w", err)
	}

	argsBytes, err := program.Int64ToBytes(memory, args)
	if err != nil {
		return nil, fmt.Errorf("failed to read program args name from memory: %w", err)
	}

	// sync args to new runtime and return arguments to the invoke call
	params, err := getCallArgs(ctx, newMem, argsBytes, ptr)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal call arguments: %w", err)
	}

	resp, err := rt.Call(ctx, string(functionBytes), params...)
	if err != nil {
		return nil, fmt.Errorf("failed to call entry function: %w", err)
	}

	return program.ValI64(resp[0]), nil
}

func getCallArgs(ctx context.Context, memory *program.Memory, buffer []byte, invokeProgramID uint32) ([]int64, error) {
	// first arg contains id of program to call
	args := []int64{int64(invokeProgramID)}
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
			resp, err := program.BytesToInt64(memory, valueBytes)
			if err != nil {
				return nil, err
			}
			args = append(args, resp)
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

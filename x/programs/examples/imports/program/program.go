// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"context"
	"encoding/binary"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/near/borsh-go"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/examples/storage"
	"github.com/ava-labs/hypersdk/x/programs/host"
	"github.com/ava-labs/hypersdk/x/programs/program"
	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

var _ host.Import = (*Import)(nil)

const Name = "program"

type Import struct {
	mu  state.Mutable
	log logging.Logger
	cfg *runtime.Config

	engine  *engine.Engine
	meter   *engine.Meter
	imports host.SupportedImports
	ctx     *program.Context
}

// New returns a new program invoke host module which can perform program to program calls.
func New(log logging.Logger, engine *engine.Engine, mu state.Mutable, cfg *runtime.Config, ctx *program.Context) *Import {
	return &Import{
		cfg:    cfg,
		mu:     mu,
		log:    log,
		engine: engine,
		ctx:    ctx,
	}
}

func (*Import) Name() string {
	return Name
}

func (i *Import) Register(link *host.Link, callContext *program.Context) error {
	i.meter = link.Meter()
	i.imports = link.Imports()

	if err := link.RegisterImportFn(Name, "call_program", i.callProgramFn(callContext)); err != nil {
		return err
	}

	if err := link.RegisterImportFn(Name, "set_call_result", i.setCallResultFn(callContext)); err != nil {
		return err
	}

	return nil
}

type callProgramFnArgs struct {
	ProgramID []byte
	Function  []byte
	Args      []byte
	MaxUnits  int64
}

// callProgramFn makes a call to an entry function of a program in the context of another program's ID.
func (i *Import) callProgramFn(_ *program.Context) func(*wasmtime.Caller, int32, int32) int32 {
	return func(
		wasmCaller *wasmtime.Caller,
		memOffset int32,
		size int32,
	) int32 {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		caller := program.NewCaller(wasmCaller)
		memory, err := caller.Memory()
		if err != nil {
			i.log.Error("failed to get memory from caller",
				zap.Error(err),
			)
			return -1
		}

		bytes, err := memory.Range(uint32(memOffset), uint32(size))
		if err != nil {
			i.log.Error("failed to read call arguments from memory",
				zap.Error(err),
			)
			return -1
		}

		args := callProgramFnArgs{}
		if err := borsh.Deserialize(&args, bytes); err != nil {
			i.log.Error("failed to unmarshal call arguments",

				zap.Error(err),
			)
			return -1
		}

		// get the program bytes from storage
		programWasmBytes, err := getProgramWasmBytes(i.log, i.mu, args.ProgramID)
		if err != nil {
			i.log.Error("failed to get program bytes from storage",
				zap.Error(err),
			)
			return -1
		}
		otherContext := &program.Context{
			ProgramID: ids.ID(args.ProgramID),
		}

		// create a new runtime for the program to be invoked with a zero balance.
		rt := runtime.New(i.log, i.engine, i.imports, i.cfg)
		err = rt.Initialize(context.Background(), otherContext, programWasmBytes, engine.NoUnits)
		if err != nil {
			i.log.Error("failed to initialize runtime",
				zap.Error(err),
			)
			return -1
		}

		// transfer the units from the caller to the new runtime before any calls are made.
		balance, err := i.meter.TransferUnitsTo(rt.Meter(), uint64(args.MaxUnits))
		if err != nil {
			i.log.Error("failed to transfer units",
				zap.Uint64("balance", balance),
				zap.Int64("required", args.MaxUnits),
				zap.Error(err),
			)
			return -1
		}

		// transfer remaining balance back to parent runtime
		defer func() {
			balance, err := rt.Meter().GetBalance()
			if err != nil {
				i.log.Error("failed to get balance from runtime",
					zap.Error(err),
				)
				return
			}
			_, err = rt.Meter().TransferUnitsTo(i.meter, balance)
			if err != nil {
				i.log.Error("failed to transfer remaining balance to caller",
					zap.Error(err),
				)
			}
		}()

		rtMemory, err := rt.Memory()
		if err != nil {
			i.log.Error("failed to get memory from runtime",
				zap.Error(err),
			)
			return -1
		}

		// sync args to new runtime and return arguments to the invoke call
		params, err := getCallArgs(ctx, rtMemory, args.Args)
		if err != nil {
			i.log.Error("failed to unmarshal call arguments",
				zap.Error(err),
			)
			return -1
		}

		functionName := string(args.Function)
		res, err := rt.Call(ctx, functionName, otherContext, params...)
		if err != nil {
			i.log.Error("failed to call entry function",
				zap.Error(err),
			)
			return -1
		}

		ptr, err := program.WriteBytes(memory, res)
		if err != nil {
			i.log.Error("failed to write result to memory",
				zap.Error(err),
			)
			return -1
		}

		return int32(ptr)
	}
}

func (i *Import) setCallResultFn(context *program.Context) func(*wasmtime.Caller, int32, int32) {
	return func(
		wasmCaller *wasmtime.Caller,
		memOffset int32,
		size int32,
	) {
		caller := program.NewCaller(wasmCaller)
		memory, err := caller.Memory()
		if err != nil {
			i.log.Error("failed to get memory from caller",
				zap.Error(err),
			)
			// TODO: panic
			return
		}

		bytes, err := memory.Range(uint32(memOffset), uint32(size))
		if err != nil {
			i.log.Error("failed to read call arguments from memory",
				zap.Error(err),
			)
			// TODO: panic
			return
		}

		context.SetResult(bytes)
	}
}

// getCallArgs returns the arguments to be passed to the program being invoked from [buffer].
func getCallArgs(_ context.Context, memory *program.Memory, buffer []byte) ([]uint32, error) {
	var args []uint32

	for i := 0; i < len(buffer); {
		// unpacks uint32
		lenBytes := buffer[i : i+consts.Uint32Len]
		length := binary.BigEndian.Uint32(lenBytes)

		valueBytes := buffer[i+consts.Uint32Len : i+consts.Uint32Len+int(length)]
		i += int(length) + consts.Uint32Len

		// every argument is a pointer
		ptr, err := program.WriteBytes(memory, valueBytes)
		if err != nil {
			return nil, err
		}
		args = append(args, ptr)
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
		return nil, errors.New("unknown program")
	}
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

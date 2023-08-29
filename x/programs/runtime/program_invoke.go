// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/utils"
)

const (
	invokeModuleName = "program"
	invokeOK         = 0
	invokeErr        = -1
)

type InvokeModule struct {
	mu      state.Mutable
	meter   Meter
	storage Storage
	log     logging.Logger
}

// NewInvokeModule returns a new program invoke host module which can perform program to program calls.
func NewInvokeModule(log logging.Logger, mu state.Mutable, meter Meter, storage Storage) *InvokeModule {
	return &InvokeModule{
		mu:      mu,
		meter:   meter,
		storage: storage,
		log:     log,
	}
}

func (m *InvokeModule) Instantiate(ctx context.Context, r wazero.Runtime) error {
	_, err := r.NewHostModuleBuilder(invokeModuleName).
		NewFunctionBuilder().WithFunc(m.programInvokeFn).Export("program_invoke").
		Instantiate(ctx)

	return err
}

// programInvokeFn makes a call to an entry function of a program in the context of another program's ID.
func (m *InvokeModule) programInvokeFn(
	ctx context.Context,
	mod api.Module,
	invokeProgramID uint64,
	entryPtr,
	entryLen,
	argsPtr,
	argsLen uint32,
) int64 {
	// get the entry function for invoke to call.
	entryBuf, ok := utils.GetBuffer(mod, entryPtr, entryLen)
	if !ok {
		m.log.Error("failed to get entry function name")
		return invokeErr
	}
	entryFn := string(entryBuf)

	// get the program bytes stored in state
	data, ok := GlobalStorage.Programs[uint32(invokeProgramID)]
	if !ok {
		m.log.Error("failed to get program bytes from storage")
		return invokeErr
	}

	// create new runtime for the program invoke call
	runtime := New(m.log, m.meter, m.storage)

	err := runtime.Initialize(ctx, data)
	if err != nil {
		m.log.Error("failed to initialize runtime for program invoke call: %v", zap.Error(err))
		return invokeErr
	}

	callArgsBuf, ok := utils.GetBuffer(mod, argsPtr, argsLen)
	if !ok {
		m.log.Error("failed to get call arguments")
		return invokeErr
	}

	// sync args to new runtime and return arguments to the invoke call
	params, err := getCallArgs(ctx, runtime, callArgsBuf, invokeProgramID)
	if err != nil {
		m.log.Error("failed to unmarshal call arguments: %v", zap.Error(err))
		return invokeErr
	}

	res, err := runtime.Call(ctx, entryFn, params...)
	if err != nil {
		m.log.Error("failed to call entry function %v", zap.Error(err))
		return invokeErr
	}
	return int64(res[0])
}

func getCallArgs(ctx context.Context, runtime Runtime, buffer []byte, invokeProgramID uint64) ([]uint64, error) {
	// first arg contains id of program to call
	args := []uint64{invokeProgramID}

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
			ptr, err := runtime.WriteGuestBuffer(ctx, valueBytes)
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

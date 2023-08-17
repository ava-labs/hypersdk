// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/x/programs/meter"
	"github.com/ava-labs/hypersdk/x/programs/utils"
)

const (
	delegateModuleName = "delegate"
	delegateOK         = 0
	delegateErr        = -1
)

type DelegateModule struct {
	db             chain.Database
	meter          meter.Meter
	programStorage ProgramStorage

	log logging.Logger
}

// NewDelegateModule returns a new delegate host module which can perform program to program calls.
func NewDelegateModule(log logging.Logger, db chain.Database, meter meter.Meter, programStorage ProgramStorage) *DelegateModule {
	return &DelegateModule{
		db:             db,
		meter:          meter,
		programStorage: programStorage,
		log:            log,
	}
}

func (d *DelegateModule) Instantiate(ctx context.Context, r wazero.Runtime) error {
	_, err := r.NewHostModuleBuilder(delegateModuleName).
		NewFunctionBuilder().WithFunc(d.delegateProgramFn).Export("delegate").
		Instantiate(ctx)

	return err
}

// delegateProgram makes a call to an entry function of a program in the context of another program's ID.
func (d *DelegateModule) delegateProgramFn(
	ctx context.Context,
	mod api.Module,
	programID,
	delegateProgramID uint64,
	entryPtr,
	entryLen,
	argsPtr,
	argsLen uint32,
) int64 {
	// get the entry function to delegate the call to
	entryBuf, ok := utils.GetBuffer(mod, entryPtr, entryLen)
	if !ok {
		return delegateErr
	}
	entryFn := utils.GetGuestFnName(string(entryBuf))

	// get the program bytes stored in state
	data, ok, err := d.programStorage.Get(ctx, uint32(programID))
	if !ok {
		return delegateErr
	}
	if err != nil {
		return delegateErr
	}

	// create new runtime for the delegated call
	runtime := New(d.log, d.meter, d.programStorage)

	// only export the function we are calling
	exportedFunctions := []string{entryFn}
	err = runtime.Initialize(ctx, data, exportedFunctions)
	if err != nil {
		return delegateErr
	}

	callArgsBuf, ok := utils.GetBuffer(mod, argsPtr, argsLen)
	if !ok {
		return delegateErr
	}

	// sync args to new runtime and return arguments to delegated call
	params, err := getCallArgs(ctx, runtime, callArgsBuf, delegateProgramID)
	if err != nil {
		return delegateErr
	}

	res, err := runtime.Call(ctx, entryFn, params...)
	if err != nil {
		return delegateErr
	}

	return int64(res[0])
}

func getCallArgs(ctx context.Context, runtime Runtime, buffer []byte, delegateProgramID uint64) ([]uint64, error) {
	// first arg contains ID of program to call
	args := []uint64{delegateProgramID}

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

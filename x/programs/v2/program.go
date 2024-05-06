// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package v2

import "C"
import (
	"context"
	"errors"
	"github.com/ava-labs/hypersdk/state"
	"github.com/bytecodealliance/wasmtime-go/v14"
)

const (
	AllocName  = "alloc"
	MemoryName = "memory"
)

type CallInfo struct {
	State           state.Mutable
	Actor           AccountID
	StateAccessList *StateAccessList
	Account         AccountID
	Program         ProgramID
	MaxUnits        uint64
	FunctionName    string
	Params          []byte
}

type ProgramID string

type Program struct {
	module    *wasmtime.Module
	programID ProgramID
}

type ProgramInstance struct {
	*Program
	inst  *wasmtime.Instance
	store *wasmtime.Store
}

func newProgram(engine *wasmtime.Engine, programID ProgramID, programBytes []byte) (*Program, error) {
	module, err := wasmtime.NewModule(engine, programBytes)
	if err != nil {
		return nil, err
	}
	return &Program{module: module, programID: programID}, nil
}

func (p *ProgramInstance) call(_ context.Context, callInfo *CallInfo) ([]byte, error) {
	// add context cancel handling
	if err := p.store.AddFuel(callInfo.MaxUnits); err != nil {
		return nil, err
	}

	//copy params into store linear memory
	offsetIntf, err := p.inst.GetExport(p.store, AllocName).Func().Call(p.store, int32(len(callInfo.Params)))
	if err != nil {
		return nil, wasmtime.NewTrap(err.Error())
	}
	offset := offsetIntf.(int32)
	programMemory := p.inst.GetExport(p.store, MemoryName).Memory()
	linearMem := programMemory.UnsafeData(p.store)
	copy(linearMem[offset:], callInfo.Params)

	resultIntf, err := p.inst.GetFunc(p.store, callInfo.FunctionName).Call(p.store, offset)
	// functions with no results or an error return nil
	if resultIntf == nil || err != nil {
		return nil, err
	}

	// functions that do have results should return the offset and length of the result bytes
	resultVals, ok := resultIntf.([]wasmtime.Val)
	if !ok || len(resultVals) != 2 {
		return nil, errors.New("functions can only return 2 values.  The first is the offset of the result struct into memory and the second is the length of the result struct.")
	}
	resultOffset := resultVals[0].I32()
	resultLength := resultVals[1].I32()

	// copy results out of memory
	resultBytes := make([]byte, 0, resultLength)
	copy(resultBytes, linearMem[resultOffset:resultOffset+resultLength])

	return resultBytes, nil
}

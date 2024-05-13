// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package v2

import "C"
import (
	"context"
	"errors"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/x/programs/program"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/near/borsh-go"
)

const (
	AllocName  = "alloc"
	MemoryName = "memory"
)

type CallInfo struct {
	State           state.Mutable
	Actor           ids.ID
	StateAccessList StateAccessList
	Account         ids.ID
	ProgramID       ids.ID
	Fuel            uint64
	FunctionName    string
	Params          []byte
	programInstance *ProgramInstance
}

type Program struct {
	module    *wasmtime.Module
	programID ids.ID
}

type ProgramInstance struct {
	*Program
	inst   *wasmtime.Instance
	store  *wasmtime.Store
	result []byte
}

func newProgram(engine *wasmtime.Engine, programID ids.ID, programBytes []byte) (*Program, error) {
	module, err := wasmtime.NewModule(engine, programBytes)
	if err != nil {
		return nil, err
	}
	return &Program{module: module, programID: programID}, nil
}

func (p *ProgramInstance) call(_ context.Context, callInfo *CallInfo) ([]byte, error) {
	if err := p.store.AddFuel(callInfo.Fuel); err != nil {
		return nil, err
	}

	// create the program context
	programCtx := program.Context{ProgramID: callInfo.ProgramID}
	programCtxBytes, err := borsh.Serialize(programCtx)
	if err != nil {
		return nil, err
	}

	//copy params into store linear memory
	ctxOffset, paramOffset, err := p.setParams(programCtxBytes, callInfo.Params)
	if err != nil {
		return nil, err
	}

	_, err = p.inst.GetFunc(p.store, callInfo.FunctionName).Call(p.store, ctxOffset, paramOffset)
	return p.result, err
}

func (p *ProgramInstance) setParams(programCtxBytes, param []byte) (int32, int32, error) {
	allocFn := p.inst.GetExport(p.store, AllocName).Func()
	programMemory := p.inst.GetExport(p.store, MemoryName).Memory()
	linearMem := programMemory.UnsafeData(p.store)

	ctxOffsetIntf, err := allocFn.Call(p.store, int32(len(programCtxBytes)))
	if err != nil {
		return 0, 0, wasmtime.NewTrap(err.Error())
	}
	ctxOffset := ctxOffsetIntf.(int32)
	copy(linearMem[ctxOffset:], programCtxBytes)

	paramOffsetIntf, err := allocFn.Call(p.store, int32(len(param)))
	if err != nil {
		return 0, 0, wasmtime.NewTrap(err.Error())
	}
	paramOffset := paramOffsetIntf.(int32)
	copy(linearMem[paramOffset:], param)

	return ctxOffset, paramOffset, nil
}

func (p *ProgramInstance) setResult(offset int32, length int32) error {
	memory := p.inst.GetExport(p.store, MemoryName).Memory()
	linearMem := memory.UnsafeData(p.store)
	if int32(len(linearMem))-(offset+length) < 0 {
		return errors.New("cannot copy more data than exists in linear memory")
	}
	p.result = make([]byte, length)
	copy(p.result, linearMem[offset:])
	return nil
}

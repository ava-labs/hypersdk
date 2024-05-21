// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package v2

import "C"

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/near/borsh-go"

	"github.com/ava-labs/hypersdk/state"
)

const (
	AllocName  = "alloc"
	MemoryName = "memory"
)

type Context struct {
	ProgramID ids.ID   `json:"program"`
	Actor     [32]byte `json:"actor"`
	// OriginatingActor [32]byte `json:"originating_actor"`
}

type CallInfo struct {
	State        state.Mutable
	Actor        ids.ID
	Account      ids.ID
	ProgramID    ids.ID
	Fuel         uint64
	FunctionName string
	Params       []byte
	inst         *ProgramInstance
}

func (c *CallInfo) RemainingFuel() uint64 {
	remaining := c.Fuel
	usedFuel, fuelEnabled := c.inst.store.FuelConsumed()
	if fuelEnabled {
		remaining -= usedFuel
	}
	return remaining
}

func (c *CallInfo) ConsumeFuel(fuel uint64) error {
	_, err := c.inst.store.ConsumeFuel(fuel)
	return err
}

type Program struct {
	module    *wasmtime.Module
	programID ids.ID
}

type ProgramInstance struct {
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
	programCtx := Context{ProgramID: callInfo.ProgramID, Actor: callInfo.Actor}
	paramsBytes, err := borsh.Serialize(programCtx)
	if err != nil {
		return nil, err
	}
	paramsBytes = append(paramsBytes, callInfo.Params...)

	// copy params into store linear memory
	paramsOffset, err := p.setParams(paramsBytes)
	if err != nil {
		return nil, err
	}

	_, err = p.inst.GetFunc(p.store, callInfo.FunctionName).Call(p.store, paramsOffset)

	return p.result, err
}

func (p *ProgramInstance) setParams(data []byte) (int32, error) {
	allocFn := p.inst.GetExport(p.store, AllocName).Func()
	programMemory := p.inst.GetExport(p.store, MemoryName).Memory()
	dataOffsetIntf, err := allocFn.Call(p.store, int32(len(data)))
	if err != nil {
		return 0, wasmtime.NewTrap(err.Error())
	}
	dataOffset := dataOffsetIntf.(int32)
	linearMem := programMemory.UnsafeData(p.store)
	copy(linearMem[dataOffset:], data)
	return dataOffset, nil
}

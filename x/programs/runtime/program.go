// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import "C"

import (
	"context"
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/hypersdk/codec"
)

const (
	AllocName  = "alloc"
	MemoryName = "memory"
)

type ProgramID []byte

type Context struct {
	Program   codec.Address
	Actor     codec.Address
	Height    uint64
	Timestamp uint64
	ActionID  ids.ID
}

type CallInfo struct {
	// the state that the program will run against
	State StateManager

	// the address that originated the initial program call
	Actor codec.Address

	// the name of the function within the program that is being called
	FunctionName string

	Program codec.Address

	// the serialized parameters that will be passed to the called function
	Params []byte

	// the maximum amount of fuel allowed to be consumed by wasm for this call
	Fuel uint64

	// the height of the chain that this call was made from
	Height uint64

	// the timestamp of the chain at the time this call was made
	Timestamp uint64

	// the action id that triggered this call
	ActionID ids.ID

	Value uint64

	inst *ProgramInstance
}

func (c *CallInfo) RemainingFuel() uint64 {
	remaining := c.Fuel
	usedFuel, fuelEnabled := c.inst.store.FuelConsumed()
	if fuelEnabled {
		remaining -= usedFuel
	}
	return remaining
}

func (c *CallInfo) AddFuel(fuel uint64) {
	// only errors if fuel isn't enable, which it always will be
	_ = c.inst.store.AddFuel(fuel)
}

func (c *CallInfo) ConsumeFuel(fuel uint64) error {
	_, err := c.inst.store.ConsumeFuel(fuel)
	return err
}

type ProgramInstance struct {
	inst   *wasmtime.Instance
	store  *wasmtime.Store
	result []byte
}

func (p *ProgramInstance) call(ctx context.Context, callInfo *CallInfo) ([]byte, error) {
	if err := p.store.AddFuel(callInfo.Fuel); err != nil {
		return nil, err
	}

	if callInfo.Value > 0 {
		if err := callInfo.State.TransferBalance(ctx, callInfo.Actor, callInfo.Program, callInfo.Value); err != nil {
			return nil, errors.New("insufficient balance")
		}
	}

	// create the program context
	programCtx := Context{
		Program:   callInfo.Program,
		Actor:     callInfo.Actor,
		Height:    callInfo.Height,
		Timestamp: callInfo.Timestamp,
		ActionID:  callInfo.ActionID,
	}
	paramsBytes, err := Serialize(programCtx)
	if err != nil {
		return nil, err
	}
	paramsBytes = append(paramsBytes, callInfo.Params...)

	// copy params into store linear memory
	paramsOffset, err := p.writeToMemory(paramsBytes)
	if err != nil {
		return nil, err
	}

	function := p.inst.GetFunc(p.store, callInfo.FunctionName)
	if function == nil {
		return nil, errors.New("this function does not exist")
	}
	_, err = function.Call(p.store, paramsOffset)

	return p.result, err
}

func (p *ProgramInstance) writeToMemory(data []byte) (int32, error) {
	allocFn := p.inst.GetExport(p.store, AllocName).Func()
	programMemory := p.inst.GetExport(p.store, MemoryName).Memory()
	dataOffsetIntf, err := allocFn.Call(p.store, int32(len(data)))
	if err != nil {
		return 0, err
	}
	dataOffset := dataOffsetIntf.(int32)
	linearMem := programMemory.UnsafeData(p.store)
	copy(linearMem[dataOffset:], data)
	return dataOffset, nil
}

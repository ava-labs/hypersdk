// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package v2

import "C"
import (
	"context"
	"errors"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/hypersdk/state"
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
	Program         ids.ID
	MaxUnits        uint64
	FunctionName    string
	Params          []byte
}

type Program struct {
	module    *wasmtime.Module
	programID ids.ID
}

type ProgramInstance struct {
	*Program
	inst  *wasmtime.Instance
	store *wasmtime.Store
}

func newProgram(engine *wasmtime.Engine, programID ids.ID, programBytes []byte) (*Program, error) {
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

	switch result := resultIntf.(type) {
	case int32, int64, float32, float64:
		{
			return borsh.Serialize(result)
		}
	case wasmtime.Val:
		{
			return borsh.Serialize(result.Get())
		}
	case []wasmtime.Val:
		{
			if len(result) != 2 {
				return nil, errors.New("functions can only return 2 values.  The first is the offset of the result struct into memory and the second is the length of the result struct.")
			}
			resultOffset := result[0].I32()
			resultLength := result[1].I32()

			// copy results out of memory
			resultBytes := make([]byte, 0, resultLength)
			copy(resultBytes, linearMem[resultOffset:resultOffset+resultLength])

			return resultBytes, nil
		}
	default:
		{
			return nil, errors.New("unknown return type")
		}
	}

}

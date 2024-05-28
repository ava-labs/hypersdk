// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"github.com/bytecodealliance/wasmtime-go/v14"
	"golang.org/x/exp/maps"
)

type Imports struct {
	Modules map[string]*ImportModule
}

type ImportModule struct {
	Name          string
	HostFunctions map[string]HostFunction
}

type HostFunction struct {
	Function HostFunctionType
	FuelCost uint64
}

type HostFunctionType interface {
	isHostFunctionType()
}

type FunctionWithOutput func(*CallInfo, []byte) ([]byte, error)

func (FunctionWithOutput) isHostFunctionType() {}

type FunctionNoOutput func(*CallInfo, []byte) error

func (FunctionNoOutput) isHostFunctionType() {}

var (
	typeI32                = wasmtime.NewValType(wasmtime.KindI32)
	functionWithOutputType = wasmtime.NewFuncType([]*wasmtime.ValType{typeI32, typeI32}, []*wasmtime.ValType{typeI32})
	FunctionNoOutputType   = wasmtime.NewFuncType([]*wasmtime.ValType{typeI32, typeI32}, []*wasmtime.ValType{})
)

func (i *ImportModule) SetFuelCost(functionName string, fuelCost uint64) bool {
	hostFunction, ok := i.HostFunctions[functionName]
	if ok {
		hostFunction.FuelCost = fuelCost
		i.HostFunctions[functionName] = hostFunction
	}

	return ok
}

func NewImports() *Imports {
	return &Imports{Modules: map[string]*ImportModule{}}
}

func (i *Imports) AddModule(mod *ImportModule) {
	i.Modules[mod.Name] = mod
}

func (i *Imports) SetFuelCost(moduleName string, functionName string, fuelCost uint64) bool {
	if module, ok := i.Modules[moduleName]; ok {
		return module.SetFuelCost(functionName, fuelCost)
	}

	return false
}

func (i *Imports) Clone() *Imports {
	return &Imports{
		Modules: maps.Clone(i.Modules),
	}
}

func (i *Imports) createLinker(engine *wasmtime.Engine, info *CallInfo) (*wasmtime.Linker, error) {
	linker := wasmtime.NewLinker(engine)
	for moduleName, module := range i.Modules {
		for funcName, hostFunction := range module.HostFunctions {
			if err := linker.FuncNew(moduleName, funcName, getFunctType(hostFunction), convertFunction(info, hostFunction)); err != nil {
				return nil, err
			}
		}
	}
	return linker, nil
}

func getFunctType(hf HostFunction) *wasmtime.FuncType {
	switch hf.Function.(type) {
	case FunctionWithOutput:
		return functionWithOutputType
	case FunctionNoOutput:
		return FunctionNoOutputType
	}
	return nil
}

var nilResult = []wasmtime.Val{wasmtime.ValI32(0)}

func convertFunction(callInfo *CallInfo, hf HostFunction) func(*wasmtime.Caller, []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
	return func(caller *wasmtime.Caller, vals []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		memExport := caller.GetExport(MemoryName)
		inputBytes := memExport.Memory().UnsafeData(caller)[vals[0].I32() : vals[0].I32()+vals[1].I32()]

		if err := callInfo.ConsumeFuel(hf.FuelCost); err != nil {
			return nil, convertToTrap(err)
		}
		switch f := hf.Function.(type) {
		case FunctionWithOutput:
			results, err := f(callInfo, inputBytes)
			if err != nil {
				return nilResult, convertToTrap(err)
			}
			if results == nil {
				return nilResult, nil
			}
			resultLength := int32(len(results))
			allocExport := caller.GetExport(AllocName)
			offsetIntf, err := allocExport.Func().Call(caller, resultLength)
			if err != nil {
				return nilResult, convertToTrap(err)
			}
			offset := offsetIntf.(int32)
			copy(memExport.Memory().UnsafeData(caller)[offset:], results)
			return []wasmtime.Val{wasmtime.ValI32(offset)}, nil
		case FunctionNoOutput:
			err := f(callInfo, inputBytes)
			if err != nil {
				return []wasmtime.Val{}, convertToTrap(err)
			}

			return []wasmtime.Val{}, nil
		default:
			return []wasmtime.Val{}, nil
		}
	}
}

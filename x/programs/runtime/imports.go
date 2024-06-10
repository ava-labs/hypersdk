// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"github.com/bytecodealliance/wasmtime-go/v14"
	"golang.org/x/exp/maps"
)

var nilResult = []wasmtime.Val{wasmtime.ValI32(0)}

type Imports struct {
	Modules map[string]*ImportModule
}

type ImportModule struct {
	Name          string
	HostFunctions map[string]HostFunction
}

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
			if err := linker.FuncNew(moduleName, funcName, hostFunction.Function.wasmType(), hostFunction.convert(info)); err != nil {
				return nil, err
			}
		}
	}
	return linker, nil
}

type HostFunction struct {
	Function HostFunctionType
	FuelCost uint64
}

func (f HostFunction) convert(callInfo *CallInfo) func(*wasmtime.Caller, []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
	convertedFunction := f.Function.convert(callInfo)
	return func(caller *wasmtime.Caller, vals []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		if err := callInfo.ConsumeFuel(f.FuelCost); err != nil {
			return nil, convertToTrap(err)
		}
		return convertedFunction(caller, vals)
	}
}

type HostFunctionType interface {
	wasmType() *wasmtime.FuncType
	convert(*CallInfo) func(*wasmtime.Caller, []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap)
}

var typeI32 = wasmtime.NewValType(wasmtime.KindI32)

type Function func(*CallInfo, []byte) ([]byte, error)

func (Function) wasmType() *wasmtime.FuncType {
	return wasmtime.NewFuncType([]*wasmtime.ValType{typeI32, typeI32}, []*wasmtime.ValType{typeI32})
}

func (f Function) convert(callInfo *CallInfo) func(*wasmtime.Caller, []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
	return func(caller *wasmtime.Caller, vals []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		results, err := f(callInfo, getInputFromMemory(caller, vals))
		return writeOutputToMemory(callInfo, results, err)
	}
}

type FunctionNoInput func(*CallInfo) ([]byte, error)

func (FunctionNoInput) wasmType() *wasmtime.FuncType {
	return wasmtime.NewFuncType([]*wasmtime.ValType{}, []*wasmtime.ValType{typeI32})
}

func (f FunctionNoInput) convert(callInfo *CallInfo) func(*wasmtime.Caller, []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
	return func(_ *wasmtime.Caller, _ []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		results, err := f(callInfo)
		return writeOutputToMemory(callInfo, results, err)
	}
}

type FunctionNoOutput func(*CallInfo, []byte) error

func (FunctionNoOutput) wasmType() *wasmtime.FuncType {
	return wasmtime.NewFuncType([]*wasmtime.ValType{typeI32, typeI32}, []*wasmtime.ValType{})
}

func (f FunctionNoOutput) convert(callInfo *CallInfo) func(*wasmtime.Caller, []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
	return func(caller *wasmtime.Caller, vals []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		err := f(callInfo, getInputFromMemory(caller, vals))
		return []wasmtime.Val{}, convertToTrap(err)
	}
}

func getInputFromMemory(caller *wasmtime.Caller, vals []wasmtime.Val) []byte {
	offset := vals[0].I32()
	length := vals[1].I32()
	if offset == 0 || length == 0 {
		return nil
	}
	return caller.GetExport(MemoryName).Memory().UnsafeData(caller)[offset : offset+length]
}

func writeOutputToMemory(callInfo *CallInfo, results []byte, err error) ([]wasmtime.Val, *wasmtime.Trap) {
	if results == nil || err != nil {
		return nilResult, convertToTrap(err)
	}
	offset, err := callInfo.inst.writeToMemory(results)
	if err != nil {
		return nilResult, convertToTrap(err)
	}
	return []wasmtime.Val{wasmtime.ValI32(offset)}, nil
}

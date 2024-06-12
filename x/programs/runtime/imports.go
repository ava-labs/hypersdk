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

func (i *Imports) createLinker(r *WasmRuntime) (*wasmtime.Linker, error) {
	linker := wasmtime.NewLinker(r.engine)
	for moduleName, module := range i.Modules {
		for funcName, hostFunction := range module.HostFunctions {
			if err := linker.FuncNew(moduleName, funcName, hostFunction.Function.wasmType(), hostFunction.convert(r)); err != nil {
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

func (f HostFunction) convert(r *WasmRuntime) func(*wasmtime.Caller, []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
	return func(caller *wasmtime.Caller, vals []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		callInfo := r.getCallInfo(caller)
		if err := callInfo.ConsumeFuel(f.FuelCost); err != nil {
			return nil, convertToTrap(err)
		}
		return f.Function.call(callInfo, caller, vals)
	}
}

type HostFunctionType interface {
	wasmType() *wasmtime.FuncType
	call(*CallInfo, *wasmtime.Caller, []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap)
}

var typeI32 = wasmtime.NewValType(wasmtime.KindI32)

type Function[T any, U any] func(*CallInfo, T) (U, error)

func (Function[T, U]) wasmType() *wasmtime.FuncType {
	return wasmtime.NewFuncType([]*wasmtime.ValType{typeI32, typeI32}, []*wasmtime.ValType{typeI32})
}

func (f Function[T, U]) call(callInfo *CallInfo, caller *wasmtime.Caller, vals []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
	input, err := getInputFromMemory[T](caller, vals)
	if err != nil {
		return writeOutputToMemory[interface{}](callInfo, nil, err)
	}
	results, err := f(callInfo, *input)
	return writeOutputToMemory(callInfo, results, err)
}

type FunctionNoInput[T any] func(*CallInfo) (T, error)

func (FunctionNoInput[T]) wasmType() *wasmtime.FuncType {
	return wasmtime.NewFuncType([]*wasmtime.ValType{}, []*wasmtime.ValType{typeI32})
}

func (f FunctionNoInput[T]) call(callInfo *CallInfo, _ *wasmtime.Caller, _ []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
	results, err := f(callInfo)
	return writeOutputToMemory[T](callInfo, results, err)
}

type FunctionNoOutput[T any] func(*CallInfo, T) error

func (FunctionNoOutput[T]) wasmType() *wasmtime.FuncType {
	return wasmtime.NewFuncType([]*wasmtime.ValType{typeI32, typeI32}, []*wasmtime.ValType{})
}

func (f FunctionNoOutput[T]) call(callInfo *CallInfo, caller *wasmtime.Caller, vals []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
	input, err := getInputFromMemory[T](caller, vals)
	if err != nil {
		return []wasmtime.Val{}, convertToTrap(err)
	}
	err = f(callInfo, *input)
	return []wasmtime.Val{}, convertToTrap(err)
}

func getInputFromMemory[T any](caller *wasmtime.Caller, vals []wasmtime.Val) (*T, error) {
	offset := vals[0].I32()
	length := vals[1].I32()

	if offset == 0 || length == 0 {
		return new(T), nil
	}
	return deserialize[T](caller.GetExport(MemoryName).Memory().UnsafeData(caller)[offset : offset+length])
}

func writeOutputToMemory[T any](callInfo *CallInfo, results T, err error) ([]wasmtime.Val, *wasmtime.Trap) {
	if err != nil {
		return nilResult, convertToTrap(err)
	}
	data, err := serialize(results)
	if data == nil || err != nil {
		return nilResult, convertToTrap(err)
	}
	offset, err := callInfo.inst.writeToMemory(data)
	if err != nil {
		return nilResult, convertToTrap(err)
	}
	return []wasmtime.Val{wasmtime.ValI32(offset)}, nil
}

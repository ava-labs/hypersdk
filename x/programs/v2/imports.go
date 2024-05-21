// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package v2

import (
	"github.com/bytecodealliance/wasmtime-go/v14"
	"golang.org/x/exp/maps"
)

type Imports struct {
	modules      map[string]*ImportModule
	totalImports int
}

type ImportModule struct {
	name  string
	funcs map[string]HostFunction
}

type HostFunction interface {
	isHostFunction()
}
type FunctionWithOutput func(*CallInfo, []byte) ([]byte, error)

func (FunctionWithOutput) isHostFunction() {}

type FunctionNoOutput func(*CallInfo, []byte) error

func (FunctionNoOutput) isHostFunction() {}

var (
	typeI32                = wasmtime.NewValType(wasmtime.KindI32)
	functionWithOutputType = wasmtime.NewFuncType([]*wasmtime.ValType{typeI32, typeI32}, []*wasmtime.ValType{typeI32})
	FunctionNoOutputType   = wasmtime.NewFuncType([]*wasmtime.ValType{typeI32, typeI32}, []*wasmtime.ValType{})
)

func NewImports() *Imports {
	return &Imports{modules: map[string]*ImportModule{}}
}

func (i *Imports) AddModule(mod *ImportModule) {
	i.modules[mod.name] = mod
	i.totalImports += len(mod.funcs)
}

func (i *Imports) Clone() *Imports {
	return &Imports{
		modules:      maps.Clone(i.modules),
		totalImports: i.totalImports,
	}
}

func (i *Imports) createLinker(engine *wasmtime.Engine, info *CallInfo) (*wasmtime.Linker, error) {
	linker := wasmtime.NewLinker(engine)
	for moduleName, module := range i.modules {
		for funcName, function := range module.funcs {
			if err := linker.FuncNew(moduleName, funcName, getFunctType(function), convertFunction(info, function)); err != nil {
				return nil, err
			}
		}
	}
	return linker, nil
}

func getFunctType(hf HostFunction) *wasmtime.FuncType {
	switch hf.(type) {
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

		switch f := hf.(type) {
		case FunctionWithOutput:
			results, err := f(callInfo, inputBytes)
			if err != nil {
				return nilResult, wasmtime.NewTrap(err.Error())
			}
			if results == nil {
				return nilResult, nil
			}
			resultLength := int32(len(results))
			allocExport := caller.GetExport(AllocName)
			offsetIntf, err := allocExport.Func().Call(caller, resultLength)
			if err != nil {
				return nilResult, wasmtime.NewTrap(err.Error())
			}
			offset := offsetIntf.(int32)
			copy(memExport.Memory().UnsafeData(caller)[offset:], results)
			return []wasmtime.Val{wasmtime.ValI32(offset)}, nil
		case FunctionNoOutput:
			err := f(callInfo, inputBytes)
			if err != nil {
				return []wasmtime.Val{}, wasmtime.NewTrap(err.Error())
			}

			return []wasmtime.Val{}, nil
		default:
			return []wasmtime.Val{}, nil
		}
	}
}

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
	funcs map[string]Function
}

type Function func(*CallInfo, []byte) ([]byte, error)

var typeI32 = wasmtime.NewValType(wasmtime.KindI32)
var functionType = wasmtime.NewFuncType([]*wasmtime.ValType{typeI32, typeI32}, []*wasmtime.ValType{typeI32})

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
			if err := linker.FuncNew(moduleName, funcName, functionType, convertFunction(info, function)); err != nil {
				return nil, err
			}
		}
	}
	return linker, nil
}

func convertFunction(callInfo *CallInfo, function Function) func(*wasmtime.Caller, []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
	return func(caller *wasmtime.Caller, vals []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		memExport := caller.GetExport(MemoryName)
		inputBytes := memExport.Memory().UnsafeData(caller)[vals[0].I32() : vals[0].I32()+vals[1].I32()]
		results, err := function(callInfo, inputBytes)
		if err != nil {
			return nil, wasmtime.NewTrap(err.Error())
		}
		if results == nil {
			return nil, nil
		}
		resultLength := int32(len(results))
		allocExport := caller.GetExport(AllocName)
		offsetIntf, err := allocExport.Func().Call(caller, resultLength)
		if err != nil {
			return nil, wasmtime.NewTrap(err.Error())
		}
		offset := offsetIntf.(int32)
		copy(memExport.Memory().UnsafeData(caller)[offset:], results)
		return []wasmtime.Val{wasmtime.ValI32(offset)}, nil
	}
}

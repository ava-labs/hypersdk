// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"context"
	"reflect"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/state"
)

type WasmRuntime struct {
	log         logging.Logger
	engine      *wasmtime.Engine
	hostImports *Imports
	cfg         *Config

	contractCache cache.Cacher[string, *wasmtime.Module]

	callerInfo                map[uintptr]*CallInfo
	linker                    *wasmtime.Linker
	linkerNeedsInitialization bool
}

type StateManager interface {
	BalanceManager
	ContractManager
}

type BalanceManager interface {
	GetBalance(ctx context.Context, address codec.Address) (uint64, error)
	TransferBalance(ctx context.Context, from codec.Address, to codec.Address, amount uint64) error
}

type ContractManager interface {
	// GetContractState returns the state of the contract at the given address.
	GetContractState(address codec.Address) state.Mutable
	// GetAccountContract returns the contract ID associated with the given account.
	// An account represents a specific instance of a contract.
	GetAccountContract(ctx context.Context, account codec.Address) (ContractID, error)
	// GetContractBytes returns the compiled WASM bytes of the contract with the given ID.
	GetContractBytes(ctx context.Context, contractID ContractID) ([]byte, error)
	// NewAccountWithContract creates a new account that represents a specific instance of a contract.
	NewAccountWithContract(ctx context.Context, contractID ContractID, accountCreationData []byte) (codec.Address, error)
	// SetAccountContract associates the given contract ID with the given account.
	SetAccountContract(ctx context.Context, account codec.Address, contractID ContractID) error
}

func NewRuntime(
	cfg *Config,
	log logging.Logger,
) *WasmRuntime {
	runtime := &WasmRuntime{
		log:                       log,
		cfg:                       cfg,
		engine:                    wasmtime.NewEngineWithConfig(cfg.wasmConfig),
		hostImports:               NewImports(),
		callerInfo:                map[uintptr]*CallInfo{},
		linkerNeedsInitialization: true,
		contractCache: cache.NewSizedLRU(cfg.ContractCacheSize, func(id string, mod *wasmtime.Module) int {
			bytes, err := mod.Serialize()
			if err != nil {
				panic(err)
			}
			return len(id) + len(bytes)
		}),
	}

	runtime.AddImportModule(NewLogModule())
	runtime.AddImportModule(NewBalanceModule())
	runtime.AddImportModule(NewStateAccessModule())
	runtime.AddImportModule(NewContractModule(runtime))

	return runtime
}

func (r *WasmRuntime) WithDefaults(callInfo CallInfo) CallContext {
	return CallContext{r: r, defaultCallInfo: callInfo}
}

func (r *WasmRuntime) AddImportModule(mod *ImportModule) {
	r.hostImports.AddModule(mod)
	r.linkerNeedsInitialization = true
}

func (r *WasmRuntime) getModule(ctx context.Context, callInfo *CallInfo, id []byte) (*wasmtime.Module, error) {
	if mod, ok := r.contractCache.Get(string(id)); ok {
		return mod, nil
	}
	contractBytes, err := callInfo.State.GetContractBytes(ctx, id)
	if err != nil {
		return nil, err
	}
	mod, err := wasmtime.NewModule(r.engine, contractBytes)
	if err != nil {
		return nil, err
	}
	r.contractCache.Put(string(id), mod)
	return mod, nil
}

func (r *WasmRuntime) CallContract(ctx context.Context, callInfo *CallInfo) (result []byte, err error) {
	contractID, err := callInfo.State.GetAccountContract(ctx, callInfo.Contract)
	if err != nil {
		return nil, err
	}
	contractModule, err := r.getModule(ctx, callInfo, contractID)
	if err != nil {
		return nil, err
	}
	inst, err := r.getInstance(contractModule, r.hostImports)
	if err != nil {
		return nil, err
	}
	callInfo.inst = inst

	r.setCallInfo(inst.store, callInfo)
	defer r.deleteCallInfo(inst.store)

	return inst.call(ctx, callInfo)
}

func (r *WasmRuntime) getInstance(contractModule *wasmtime.Module, imports *Imports) (*ContractInstance, error) {
	if r.linkerNeedsInitialization {
		linker, err := imports.createLinker(r)
		if err != nil {
			return nil, err
		}
		r.linker = linker
		r.linkerNeedsInitialization = false
	}

	store := wasmtime.NewStore(r.engine)
	store.SetEpochDeadline(1)
	inst, err := r.linker.Instantiate(store, contractModule)
	if err != nil {
		return nil, err
	}
	return &ContractInstance{inst: inst, store: store}, nil
}

func toMapKey(storeLike wasmtime.Storelike) uintptr {
	return reflect.ValueOf(storeLike.Context()).Pointer()
}

func (r *WasmRuntime) setCallInfo(storeLike wasmtime.Storelike, info *CallInfo) {
	r.callerInfo[toMapKey(storeLike)] = info
}

func (r *WasmRuntime) getCallInfo(storeLike wasmtime.Storelike) *CallInfo {
	return r.callerInfo[toMapKey(storeLike)]
}

func (r *WasmRuntime) deleteCallInfo(storeLike wasmtime.Storelike) {
	delete(r.callerInfo, toMapKey(storeLike))
}

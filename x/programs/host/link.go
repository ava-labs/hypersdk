// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package host

import (
	"errors"
	"fmt"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/bytecodealliance/wasmtime-go/v14"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/program"
	"github.com/ava-labs/hypersdk/x/programs/program/types"
)

var ErrMissingImportModule = errors.New("failed to find import module")

// NewLink returns a new host module program link.
func NewLink(log logging.Logger, engine *wasmtime.Engine, imports SupportedImports, meter *engine.Meter, debugMode bool) *Link {
	return &Link{
		log:       log,
		wasmLink:  wasmtime.NewLinker(engine),
		imports:   imports,
		meter:     meter,
		debugMode: debugMode,
	}
}

type Link struct {
	wasmLink  *wasmtime.Linker
	meter     *engine.Meter
	imports   SupportedImports
	log       logging.Logger
	debugMode bool

	// cb is a global callback for import function requests and responses.
	cb ImportFnCallback
}

// Instantiate registers a module with all imports defined in this linker.
// This can only be called once after all imports have been registered.
func (l *Link) Instantiate(store *engine.Store, mod *wasmtime.Module, cb ImportFnCallback, callContext *program.Context) (*wasmtime.Instance, error) {
	l.cb = cb
	if l.debugMode {
		err := l.EnableWasi()
		if err != nil {
			return nil, err
		}
	}

	imports := getRegisteredImports(mod.Imports())
	// register host functions exposed to the guest (imports)
	for _, imp := range imports {
		importFn, ok := l.imports[imp]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrMissingImportModule, imp)
		}
		err := importFn().Register(l, callContext)
		if err != nil {
			return nil, err
		}
	}
	return l.wasmLink.Instantiate(store.Get(), mod)
}

// Meter returns the meter for the module the link is linking to.
func (l *Link) Meter() *engine.Meter {
	return l.meter
}

// Imports returns the supported imports for the link instance.
func (l *Link) Imports() SupportedImports {
	return l.imports
}

// RegisterImportFn registers a host function exposed to the guest (import).
func (l *Link) RegisterImportFn(module, name string, f interface{}) error {
	wrapper := func() interface{} {
		if l.cb.BeforeRequest != nil {
			err := l.cb.BeforeRequest(module, name, l.meter)
			if err != nil {
				l.log.Error("before request callback failed",
					zap.Error(err),
				)
			}
		}
		if l.cb.AfterResponse != nil {
			defer func() {
				err := l.cb.AfterResponse(module, name, l.meter)
				if err != nil {
					l.log.Error("after response callback failed",
						zap.Error(err),
					)
				}
			}()
		}
		return f
	}
	return l.wasmLink.FuncWrap(module, name, wrapper())
}

// RegisterImportWrapFn registers a wrapped host function exposed to the guest (import). RegisterImportWrapFn allows for
// more control over the function wrapper than RegisterImportFn.
func (l *Link) RegisterImportWrapFn(module, name string, paramCount int, f func(caller *program.Caller, args ...wasmtime.Val) (*types.Val, error)) error {
	fn := func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		if l.cb.BeforeRequest != nil {
			err := l.cb.BeforeRequest(module, name, l.meter)
			if err != nil {
				// fail fast
				return nil, wasmtime.NewTrap(err.Error())
			}
		}
		if l.cb.AfterResponse != nil {
			defer func() {
				err := l.cb.AfterResponse(module, name, l.meter)
				if err != nil {
					l.log.Error("after response callback failed",
						zap.Error(err),
					)
					l.wasmLink.Engine.IncrementEpoch()
				}
			}()
		}

		val, err := f(program.NewCaller(caller), args...)
		if err != nil {
			return nil, wasmtime.NewTrap(err.Error())
		}

		return []wasmtime.Val{val.Wasmtime()}, nil
	}

	// TODO: support other types?
	valType := make([]*wasmtime.ValType, paramCount)
	for i := 0; i < paramCount; i++ {
		valType[i] = wasmtime.NewValType(wasmtime.KindI32)
	}

	funcType := wasmtime.NewFuncType(
		valType,
		[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI32)},
	)

	return l.wasmLink.FuncNew(module, name, funcType, fn)
}

// EnableWasi enables wasi support for the link.
func (l *Link) EnableWasi() error {
	return l.wasmLink.DefineWasi()
}

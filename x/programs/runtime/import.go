// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

// SupportedImports is a map of supported import modules. The runtime will enable these imports
// during initialization only if implemented by the `program`.
type SupportedImports map[string]func() Import

type Supported struct {
	imports SupportedImports
}

func NewSupportedImports() *Supported {
	return &Supported{
		imports: make(SupportedImports),
	}
}

// Register registers a supported import by name.
func (s *Supported) Register(name string, f func() Import) *Supported {
	s.imports[name] = f
	return s
}

// Imports returns the supported imports.
func (s *Supported) Imports() SupportedImports {
	return s.imports
}

// Factory is a factory for creating imports.
type Factory struct {
	log               logging.Logger
	registeredImports Imports
	supportedImports  SupportedImports
}

func NewImportFactory(log logging.Logger, supported SupportedImports) *Factory {
	return &Factory{
		log:               log,
		supportedImports:  supported,
		registeredImports: make(Imports),
	}
}

// Register registers a supported import by name.
func (f *Factory) Register(name string) *Factory {
	f.registeredImports[name] = f.supportedImports[name]()
	return f
}

// Imports returns the registered imports.
func (f *Factory) Imports() Imports {
	return f.registeredImports
}

type importFnCallback struct {
	// beforeRequest is called before the import function request is made.
	beforeRequest func(module, name string) error
	// afterResponse is called after the import function response is received.
	afterResponse func(module, name string) error
}

// NewLink returns a new host module link.
func NewLink(engine *wasmtime.Engine) *Link {
	return &Link{
		inner: wasmtime.NewLinker(engine),
	}
}

type Link struct {
	inner *wasmtime.Linker
	cb    *importFnCallback
}

// Instantiate instantiates a module with all imports defined in this linker.
func (l *Link) Instantiate(store wasmtime.Storelike, module *wasmtime.Module) (*wasmtime.Instance, error) {
	return l.inner.Instantiate(store, module)
}

// RegisterFn registers a host function exposed to the guest (import).
func (l *Link) RegisterFn(module, name string, f interface{}) error {
	val := reflect.ValueOf(f)
	funcType := val.Type()
	if funcType.NumOut() == 0 {
		return fmt.Errorf("%w: host functions must return a value", ErrInvalidFunction)
	}

	wrapper := func(args []reflect.Value) []reflect.Value {
		if l.cb.beforeRequest != nil {
			err := l.cb.beforeRequest(module, name)
			if err != nil {
				// fail fast
				return ensureErrorResult(funcType)
			}
		}
		if l.cb.afterResponse != nil {
			defer l.cb.afterResponse(module, name)
		}

		return val.Call(args)
	}
	wrappedFn := reflect.MakeFunc(val.Type(), wrapper)

	return l.inner.FuncWrap(module, name, wrappedFn.Interface())
}

func (l *Link) registerCallback(cb *importFnCallback) {
	l.cb = cb
}

func (l *Link) wasi() error {
	return l.inner.DefineWasi()
}

// TODO: return error code to guest program that will result in guaranteed panic.
func ensureErrorResult(funcType reflect.Type) []reflect.Value {
	switch funcType.Out(0).Kind() {
	case reflect.Int32:
		return []reflect.Value{reflect.ValueOf(int32(-1))}
	case reflect.Int64:
		return []reflect.Value{reflect.ValueOf(int64(-1))}
	default:
		panic(fmt.Sprintf("unsupported return type: %s", funcType.Out(0).Kind()))
	}
}

func errorToResult(err error) []reflect.Value {
	switch {
		case errors.Is(err, ErrNotFound):
	}
	if err != nil {
		return ensureErrorResult(reflect.TypeOf(err))
	}
	return []reflect.Value{}
}
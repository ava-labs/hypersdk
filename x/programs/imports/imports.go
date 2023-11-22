// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package imports

import (
	"fmt"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

type Imports map[string]Import

type Callback struct {
	// beforeRequest is called before the import function request is made.
	BeforeRequest func(module, name string) error
	// afterResponse is called after the import function response is received.
	AfterResponse func(module, name string) error
}

// Supported is a map of supported import modules. The runtime will enable these imports
// during initialization only if implemented by the `program`.
type Supported map[string]func() Import

type Builder struct {
	imports map[string]func() Import
}

func NewBuilder() *Builder {
	return &Builder{
		imports: make(map[string]func() Import),
	}
}

// Register registers a supported import by name.
func (s *Builder) Register(name string, f func() Import) *Builder {
	s.imports[name] = f
	return s
}

// Imports returns the supported imports.
func (s *Builder) Build() Supported {
	return s.imports
}

// Factory is a factory for creating imports.
type Factory struct {
	log               logging.Logger
	registeredImports Imports
	supportedImports  Supported
}

// NewImportFactory returns a new import factory which can register supported
// imports for a program.
func NewFactory(log logging.Logger, supported map[string]func() Import) *Factory {
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

type ImportFnCallback struct {
	// beforeRequest is called before the import function request is made.
	BeforeRequest func(module, name string) error
	// afterResponse is called after the import function response is received.
	AfterResponse func(module, name string) error
}

type FnTypes interface {
	OneParamFn | TwoParamFn | ThreeParamFn | FourParamFn | FiveParamFn | SixParamFn
}

// FnType is an interface for import functions with variadic int64 arguments
type FnType interface {
	Invoke(Caller, ...int64) (Val, error)
}

// ImportFn is a generic type that satisfies ImportFnType
type Fn[F FnTypes] struct {
	fn F
}

// Invoke calls the underlying function with the given arguments. Currently only
// supports int64 arguments and return values.
func (i Fn[F]) Invoke(c Caller, args ...int64) (Val, error) {
	switch fn := any(i.fn).(type) {
	case OneParam:
		return fn.Call(c, args[0])
	case TwoParam:
		return fn.Call(c, args[0], args[1])
	case ThreeParam:
		return fn.Call(c, args[0], args[1], args[2])
	case FourParam:
		return fn.Call(c, args[0], args[1], args[2], args[3])
	case FiveParam:
		return fn.Call(c, args[0], args[1], args[2], args[3], args[4])
	case SixParam:
		return fn.Call(c, args[0], args[1], args[2], args[3], args[4], args[5])
	default:
		return Val{}, fmt.Errorf("unsupported")
	}
}

func NewFnBuilder[F FnTypes]() *FnBuilder[F] {
	return &FnBuilder[F]{}
}

// Generic Builder for ImportFn[T]
type FnBuilder[F FnTypes] struct {
	fn F
}

func (b *FnBuilder[F]) SetFn(fn F) *FnBuilder[F] {
	b.fn = fn
	return b
}

func (b *FnBuilder[F]) Build() func(caller Caller, wargs ...wasmtime.Val) (Val, error) {
	importFn := Fn[F]{fn: b.fn}
	fn := func(c Caller, wargs ...wasmtime.Val) (Val, error) {
		args := make([]int64, 0, len(wargs))
		for _, arg := range wargs {
			args = append(args, int64(arg.I64()))
		}
		return importFn.Invoke(c, args...)
	}
	return fn
}

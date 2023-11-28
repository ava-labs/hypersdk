// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package host

import (
	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/x/programs/engine"
)

const (
	wasiPreview1ModName = "wasi_snapshot_preview1"
)

type Imports map[string]Import

type Callback struct {
	// beforeRequest is called before the import function request is made.
	BeforeRequest func(module, name string, meter engine.Meter) error
	// afterResponse is called after the import function response is received.
	AfterResponse func(module, name string, meter engine.Meter) error
}

// Supported is a map of supported import modules. The runtime will enable these imports
// during initialization only if implemented by the `program`.
type SupportedImports map[string]func() Import

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
func (s *Builder) Build() SupportedImports {
	return s.imports
}

// Factory is a factory for creating imports.
type Factory struct {
	log               logging.Logger
	registeredImports Imports
	supportedImports  SupportedImports
}

// NewFactory returns a new import factory which can register supported
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

type FnCallback struct {
	// beforeRequest is called before the import function request is made.
	BeforeRequest func(module, name string) error
	// afterResponse is called after the import function response is received.
	AfterResponse func(module, name string) error
}

// getRegisteredImports returns the unique names of all import modules registered
// by the wasm module.
func getRegisteredImports(importTypes []*wasmtime.ImportType) []string {
	u := make(map[string]struct{}, len(importTypes))
	imports := []string{}
	for _, t := range importTypes {
		mod := t.Module()
		if mod == wasiPreview1ModName {
			continue
		}
		if _, ok := u[mod]; ok {
			continue
		}
		u[mod] = struct{}{}
		imports = append(imports, mod)
	}
	return imports
}
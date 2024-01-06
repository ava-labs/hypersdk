// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package host

import (
	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/hypersdk/x/programs/engine"
)

const (
	wasiPreview1ModName = "wasi_snapshot_preview1"
)

var NoSupportedImports = make(SupportedImports)

type ImportFnCallback struct {
	// beforeRequest is called before the import function request is made.
	BeforeRequest func(module, name string, meter *engine.Meter) error
	// afterResponse is called after the import function response is received.
	AfterResponse func(module, name string, meter *engine.Meter) error
}

// Supported is a map of supported import modules. The runtime will enable these imports
// during initialization only if implemented by the `program`.
type SupportedImports map[string]func() Import

type ImportsBuilder struct {
	imports map[string]func() Import
}

func NewImportsBuilder() *ImportsBuilder {
	return &ImportsBuilder{
		imports: make(map[string]func() Import),
	}
}

// Register registers a supported import by name.
func (s *ImportsBuilder) Register(name string, f func() Import) *ImportsBuilder {
	s.imports[name] = f
	return s
}

// Build returns the supported imports.
func (s *ImportsBuilder) Build() SupportedImports {
	return s.imports
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

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"github.com/ava-labs/avalanchego/utils/logging"
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

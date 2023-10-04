package runtime

import (
	"github.com/ava-labs/avalanchego/utils/logging"
)

// SupportedImports is a map of supported import modules. The runtime will enable these imports
// when the runtime is initialized only if implemented by the `program`.
type SupportedImports map[string]func() Import

func NewSupportedImports() SupportedImports {
	return make(SupportedImports)
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

package host

import (
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"

	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/x/programs/engine"
	"github.com/ava-labs/hypersdk/x/programs/program"
)

// NewLink returns a new host module program link.
func NewLink(log logging.Logger, engine *wasmtime.Engine, imports SupportedImports, meter engine.Meter, cfg *engine.Config) *Link {
	return &Link{
		log:     log,
		inner:   wasmtime.NewLinker(engine),
		imports: imports,
		meter:   meter,
		cfg:     cfg,
	}
}

type Link struct {
	inner   *wasmtime.Linker
	imports SupportedImports
	log     logging.Logger
	meter   engine.Meter
	cfg     *engine.Config
	cb      ImportFnCallback
}

// Instantiate registers a module with all imports defined in this linker.
// This can only be called once after all imports have been registered.
func (l *Link) Instantiate(store wasmtime.Storelike, mod *wasmtime.Module) (*wasmtime.Instance, error) {
	imports := getRegisteredImports(mod.Imports())
	// register host functions exposed to the guest (imports)
	for _, imp := range imports {
		importFn, ok := l.imports[imp]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrMissingImportModule, imp)
		}
		err := importFn().Register(l)
		if err != nil {
			return nil, err
		}
	}
	return l.inner.Instantiate(store, mod)
}

// Meter returns the meter for the module the link is linking to.
func (l *Link) Meter() engine.Meter {
	return l.meter
}

// RegisterFn registers a host function exposed to the guest (import).
func (l *Link) RegisterFn(module, name string, paramCount int, f func(c *program.Caller, args ...wasmtime.Val) (*program.Val, error)) error {
	fn := func(caller *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		if l.cb.BeforeRequest != nil {
			err := l.cb.BeforeRequest(module, name, l.meter)
			if err != nil {
				// fail fast
				return nil, wasmtime.NewTrap(err.Error())
			}
		}
		if l.cb.AfterResponse != nil {
			defer l.cb.AfterResponse(module, name, l.meter)
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
		valType[i] = wasmtime.NewValType(wasmtime.KindI64)
	}

	funcType := wasmtime.NewFuncType(
		valType,
		[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI64)},
	)

	return l.inner.FuncNew(module, name, funcType, fn)
}

func (l *Link) RegisterFuncWrap(module, name string, f interface{}) error {
	wrapper := func() interface{} {
		if l.cb.BeforeRequest != nil {
			err := l.cb.BeforeRequest(module, name, l.meter)
			if err != nil {
				l.inner.Engine.IncrementEpoch()
			}
		}
		if l.cb.AfterResponse != nil {
			defer l.cb.AfterResponse(module, name, l.meter)
		}
		return f
	}
	return l.inner.FuncWrap(module, name, wrapper())
}

// RegisterCallback registers a callback for import function requests and responses.
func (l *Link) RegisterCallback(cb ImportFnCallback) {
	l.cb = cb
}

// Wasi enables wasi support for the link.
func (l *Link) Wasi() error {
	return l.inner.DefineWasi()
}

// Log returns a logger exposed by the link.
func (l *Link) Log() logging.Logger {
	return l.log
}
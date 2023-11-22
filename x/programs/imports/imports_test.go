package imports

import (
	"testing"

	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/x/programs/runtime"
)

type Caller struct {
	inner *wasmtime.Caller
}

func NewCaller(inner *wasmtime.Caller) *Caller {
	return &Caller{
		inner: inner,
	}
}

func (c *Caller) Call(module, name string, args ...interface{}) ([]wasmtime.Val, *wasmtime.Trap) {
	return c.inner.Call(module, name, args...)
}

func (c *Caller) GetExport(module, name string) *wasmtime.Extern {
	return c.inner.GetExport(module, name)
}

func (c *Caller) GetExportFunc(module, name string) (*wasmtime.Func, error) {
	return c.inner.GetExportFunc(module, name)
}

func (c *Caller) GetExportGlobal(module, name string) (*wasmtime.Global, error) {
	return c.inner.GetExportGlobal(module, name)
}

func TestXxx(t *testing.T) {
	require := require.New(t)

	cfg, err := runtime.NewConfigBuilder().Build()
	require.NoError(err)

	engineConfig, err := cfg.Engine()
	require.NoError(err)

	store := wasmtime.NewStore(wasmtime.NewEngineWithConfig(engineConfig))

	

	f := func(c *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
		builder := NewFnBuilder[NewCaller(c), ThreeParam],
		return []wasmtime.Val{}, nil
	}

	tt := wasmtime.NewFunc(
		store,
		wasmtime.NewFuncType(
			[]*wasmtime.ValType{
				wasmtime.NewValType(wasmtime.KindI64),
				wasmtime.NewValType(wasmtime.KindI64),
			},
			[]*wasmtime.ValType{wasmtime.NewValType(wasmtime.KindI64)},
		),
		f,
	)

	tttt := func(program *Program, args interface{}, f wasmtime.NewFunc) ([]Val, *wasmtime.Trap) {

	}

}

type Args struct {
	data []byte
}

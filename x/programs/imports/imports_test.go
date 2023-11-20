package imports

import (
	"testing"

	"github.com/bytecodealliance/wasmtime-go/v14"
	"github.com/stretchr/testify/require"
)

func TestXxx(t *testing.T) {
	require := require.New(t)

	cfg, err := NewConfigBuilder().Build()
	require.NoError(err)

	engineConfig, err := cfg.Engine()
	require.NoError(err)

	store := wasmtime.NewStore(wasmtime.NewEngineWithConfig(engineConfig))

	f := func(c *wasmtime.Caller, args []wasmtime.Val) ([]wasmtime.Val, *wasmtime.Trap) {
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

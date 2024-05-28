package runtime

import (
	"errors"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

func convertToTrap(err error) *wasmtime.Trap {
	var t *wasmtime.Trap
	switch {
	case errors.As(err, &t):
		return t
	default:
		return wasmtime.NewTrap(err.Error())
	}
}

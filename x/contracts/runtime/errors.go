// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"errors"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

func convertToTrap(err error) *wasmtime.Trap {
	if err == nil {
		return nil
	}
	var t *wasmtime.Trap
	switch {
	case errors.As(err, &t):
		return t
	default:
		return wasmtime.NewTrap(err.Error())
	}
}

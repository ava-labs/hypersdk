// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package program

import (
	"errors"
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

var (
	ErrMissingExportedFunction      = errors.New("failed to find exported function")
	ErrMissingInvalidMemoryFunction = errors.New("memory function is invalid")
	ErrRuntimeStoreSet              = errors.New("runtime store has already been set")
	ErrOverflow                     = errors.New("overflow")
	ErrUnderflow                    = errors.New("underflow")
	ErrInvalidType                  = errors.New("invalid type")
	ErrNegativeValue                = errors.New("negative value")

	// Memory
	ErrInvalidMemorySize    = errors.New("invalid memory size")
	ErrInvalidMemoryAddress = errors.New("invalid memory address: must be positive")

	// Func
	ErrInvalidParamCount = errors.New("invalid param count")
	ErrInvalidParamType  = errors.New("invalid param type")

	// Trap errors
	ErrTrapStackOverflow          = errors.New("the current stack space was exhausted")
	ErrTrapMemoryOutOfBounds      = errors.New("out-of-bounds memory access")
	ErrTrapHeapMisaligned         = errors.New("a wasm atomic operation was presented with a not-naturally-aligned linear-memory address")
	ErrTrapTableOutOfBounds       = errors.New("out-of-bounds table access")
	ErrTrapIndirectCallToNull     = errors.New("an indirect call to a null table entry was executed")
	ErrTrapIndirectCallBadSig     = errors.New("an indirect call signature mismatch was detected")
	ErrTrapBadSignature           = errors.New("signature mismatch on indirect call")
	ErrTrapIntegerOverflow        = errors.New("an integer arithmetic operation caused an overflow")
	ErrTrapIntegerDivisionByZero  = errors.New("an integer divide-by-zero was executed")
	ErrTrapBadConversionToInteger = errors.New("failed float-to-int conversion")
	ErrTrapUnreachableCodeReached = errors.New("code that was supposed to have been unreachable was reached")
	ErrTrapInterrupt              = errors.New("an interrupt was received")
	ErrTrapOutOfFuel              = errors.New("the program ran out of units")
)

// HandleTrapError returns the error message from a wasmtime Trap
func HandleTrapError(err error) error {
	trap, ok := err.(*wasmtime.Trap)
	if !ok {
		return fmt.Errorf("error is not a wasmtime.Trap error: %w", err)
	}

	var trapErr error
	code := trap.Code()
	switch *code {
	case 0:
		trapErr = ErrTrapStackOverflow
	case 1:
		trapErr = ErrTrapMemoryOutOfBounds
	case 2:
		trapErr = ErrTrapHeapMisaligned
	case 3:
		trapErr = ErrTrapTableOutOfBounds
	case 4:
		trapErr = ErrTrapIndirectCallToNull
	case 5:
		trapErr = ErrTrapBadSignature
	case 6:
		trapErr = ErrTrapIntegerOverflow
	case 7:
		trapErr = ErrTrapIntegerDivisionByZero
	case 8:
		trapErr = ErrTrapBadConversionToInteger
	case 9:
		trapErr = ErrTrapUnreachableCodeReached
	case 10:
		trapErr = ErrTrapInterrupt
	case 11:
		trapErr = ErrTrapOutOfFuel
	default:
		return fmt.Errorf("unknown runtime engine trap: %v", *code)
	}

	return fmt.Errorf("%w", trapErr)
}

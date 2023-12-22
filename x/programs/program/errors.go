// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import (
	"errors"
	"fmt"

	"github.com/bytecodealliance/wasmtime-go/v14"
)

var (
	ErrMissingExportedFunction      = errors.New("failed to find exported function")
	ErrMissingImportModule          = errors.New("failed to find import module")
	ErrMissingInvalidMemoryFunction = errors.New("memory function is invalid")
	ErrInvalidMemorySize            = errors.New("invalid memory size")
	ErrInvalidMemoryAddress         = errors.New("invalid memory address: must be positive")
	ErrInvalidParamCount            = errors.New("invalid parameter count")
	ErrInvalidParamType             = errors.New("invalid parameter type")
	ErrRuntimeStoreSet              = errors.New("runtime store has already been set")
	ErrNegativeValue                = errors.New("negative value")
	ErrIntegerConversionOverflow    = errors.New("integer overflow during conversion")
	ErrOverflow                     = errors.New("overflow")
	ErrUnderflow                    = errors.New("underflow")

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

// handleTrapError returns the error message from a wasmtime Trap
func handleTrapError(err error) error {
	trap, ok := err.(*wasmtime.Trap)
	if !ok {
		return err
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
		trapErr = ErrTrapIndirectCallBadSig
	case 6:
		trapErr = ErrTrapBadSignature
	case 7:
		trapErr = ErrTrapIntegerOverflow
	case 8:
		trapErr = ErrTrapIntegerDivisionByZero
	case 9:
		trapErr = ErrTrapBadConversionToInteger
	case 10:
		trapErr = ErrTrapUnreachableCodeReached
	case 11:
		trapErr = ErrTrapInterrupt
	case 12:
		trapErr = ErrTrapOutOfFuel
	default:
		return fmt.Errorf("unknown runtime engine trap: %v", *code)
	}

	return fmt.Errorf("%w", trapErr)
}

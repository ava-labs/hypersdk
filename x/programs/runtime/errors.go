// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package runtime

import "errors"

var (
	ErrMissingExportedFunction      = errors.New("failed to find exported function")
	ErrMissingImportModule          = errors.New("failed to find import module")
	ErrMissingInvalidMemoryFunction = errors.New("memory function is invalid")
	ErrInvalidMemorySize            = errors.New("invalid memory size")
	ErrInvalidMemoryAddress         = errors.New("invalid memory address: must be positive")
	ErrInvalidParamCount            = errors.New("invalid parameter count")
	ErrInvalidParamType             = errors.New("invalid parameter type")
	ErrInsufficientUnits            = errors.New("insufficient units")
)

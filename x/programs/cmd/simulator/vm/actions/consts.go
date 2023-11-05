// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package actions

// Note: Registry will error during initialization if a duplicate ID is assigned. We explicitly assign IDs to avoid accidental remapping.
const (
	programCreateID  uint8 = 0
	programExecuteID uint8 = 1
)

const (
	ProgramCreateComputeUnits  = 1
	ProgramExecuteComputeUnits = 1
)

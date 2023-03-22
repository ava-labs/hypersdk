// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"errors"
)

var (
	ErrNotAdded       = errors.New("not added")
	ErrDropped        = errors.New("dropped")
	ErrNotReady       = errors.New("not ready")
	ErrStateMissing   = errors.New("state missing")
	ErrMessageMissing = errors.New("message missing")
	ErrStateSyncing   = errors.New("state still syncing")
)

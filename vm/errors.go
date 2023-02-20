// Copyright (C) 2019-2021, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"errors"
)

var (
	ErrNotAdded     = errors.New("not added")
	ErrDropped      = errors.New("dropped")
	ErrNotReady     = errors.New("not ready")
	ErrStateMissing = errors.New("state missing")
)

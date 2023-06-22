// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package genesis

import "errors"

var (
	ErrInvalidTarget      = errors.New("invalid target")
	ErrStateLockupMissing = errors.New("state lockup parameter missing")
)

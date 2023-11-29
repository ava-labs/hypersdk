// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package host

import (
	"errors"
)

var (
	ErrInstantiated        = errors.New("link already instantiated")
	ErrMissingImportModule = errors.New("failed to find import module")
)

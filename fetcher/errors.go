// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package fetcher

import "errors"

var (
	ErrInvalidKeyValue = errors.New("invalid key or value")
	ErrStopped         = errors.New("stopped")
)

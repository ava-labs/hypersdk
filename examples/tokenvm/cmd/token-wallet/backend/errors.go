// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package backend

import "errors"

var (
	ErrDuplicate    = errors.New("duplicate")
	ErrAssetMissing = errors.New("asset missing")
)

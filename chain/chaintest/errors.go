// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chaintest

import "errors"

var (
	ErrMismatchedKeysAndFactoriesLen = errors.New("mismatched keys and factories len")
	ErrSingleKeyLengthOnly           = errors.New("single key length only")
)

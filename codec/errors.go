// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package codec

import "errors"

var (
	ErrTooManyItems      = errors.New("too many items")
	ErrDuplicateItem     = errors.New("duplicate item")
	ErrFieldNotPopulated = errors.New("field is not populated")
)

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import "errors"

var (
	ErrContentAlreadyExists = errors.New("content already exists")
	ErrContentMissing       = errors.New("content does not exist")
	ErrWrongOwner           = errors.New("wrong owner")
	ErrInsufficientTip      = errors.New("insufficient tip")
	ErrInvalidBalance       = errors.New("invalid balance")
)

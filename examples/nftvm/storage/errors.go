// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package storage

import "errors"

var ErrInvalidBalance = errors.New("invalid balance")

var (
	ErrOrderAlreadyExists                  = errors.New("order already exists")
	ErrCorruptInstanceMarketplaceIndicator = errors.New("instance marketplace indicator neither one nor zero")
)

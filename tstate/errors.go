// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import "errors"

var (
	ErrNewKeysDisabled    = errors.New("new keys disabled")
	ErrKeyNotSpecified    = errors.New("key not specified")
	ErrInvalidKeyValue    = errors.New("invalid key or value")
	ErrAllocationDisabled = errors.New("allocation disabled")
)

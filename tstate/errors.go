// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tstate

import "errors"

var (
	ErrNewPermissionsDisabled = errors.New("new keys disabled")
	ErrInvalidKeyOrPermission = errors.New("invalid key or key permission")
	ErrInvalidKeyValue        = errors.New("invalid key or value")
	ErrAllocationDisabled     = errors.New("allocation disabled")
)

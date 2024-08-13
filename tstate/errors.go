package tstate

import "errors"

var (
	ErrNewPermissionsDisabled = errors.New("new keys disabled")
	ErrInvalidKeyOrPermission = errors.New("invalid key or key permission")
	ErrInvalidKeyValue        = errors.New("invalid key or value")
	ErrAllocationDisabled     = errors.New("allocation disabled")
)

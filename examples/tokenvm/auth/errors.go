// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package auth

import "errors"

var (
	ErrInvalidSignature  = errors.New("invalid signature")
	ErrActorMissing      = errors.New("actor account is missing")
	ErrNotAllowed        = errors.New("not allowed")
	ErrActionMissing     = errors.New("action missing")
	ErrActorCantPay      = errors.New("actor can't pay")
	ErrActorEqualsSigner = errors.New("actor equals signer, use direct")
)

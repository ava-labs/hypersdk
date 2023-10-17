// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package crypto

import "errors"

var (
	ErrInvalidPrivateKey = errors.New("invalid private key")
	ErrInvalidPublicKey  = errors.New("invalid public key")
	ErrIncorrectHrp      = errors.New("incorrect hrp")
	ErrInvalidSignature  = errors.New("invalid signature")
)

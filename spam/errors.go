// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package spam

import "errors"

var (
	// ErrInputEmpty          = errors.New("input is empty")
	// ErrInputTooLarge       = errors.New("input is too large")
	// ErrInvalidChoice       = errors.New("invalid choice")
	// ErrIndexOutOfRange     = errors.New("index out-of-range")
	// ErrInsufficientBalance = errors.New("insufficient balance")
	// ErrDuplicate           = errors.New("duplicate")
	// ErrNoChains            = errors.New("no available chains")
	ErrNoKeys              = errors.New("no available keys")
	ErrInvalidInputs	   = errors.New("invalid inputs")
	ErrInvalidKeyType    = errors.New("invalid key type")
	// ErrTxFailed            = errors.New("tx failed on-chain")
)

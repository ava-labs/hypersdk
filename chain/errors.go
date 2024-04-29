// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package chain

import "errors"

var (
	// Parsing
	ErrInvalidObject = errors.New("invalid object")

	// Genesis Correctness
	ErrInvalidChainID   = errors.New("invalid chain ID")
	ErrInvalidBlockRate = errors.New("invalid block rate")

	// Block Correctness
	ErrTimestampTooEarly    = errors.New("timestamp too early")
	ErrTimestampTooLate     = errors.New("timestamp too late")
	ErrStateRootEmpty       = errors.New("state root empty")
	ErrNoTxs                = errors.New("no transactions")
	ErrInvalidFee           = errors.New("invalid fee")
	ErrInvalidUnitWindow    = errors.New("invalid unit window")
	ErrInvalidBlockCost     = errors.New("invalid block cost")
	ErrInvalidBlockWindow   = errors.New("invalid block window")
	ErrInvalidUnitsConsumed = errors.New("invalid units consumed")
	ErrInsufficientSurplus  = errors.New("insufficient surplus fee")
	ErrInvalidSurplus       = errors.New("invalid surplus fee")
	ErrStateRootMismatch    = errors.New("state root mismatch")
	ErrInvalidResult        = errors.New("invalid result")
	ErrInvalidBlockHeight   = errors.New("invalid block height")

	// Tx Correctness
	ErrInvalidSignature     = errors.New("invalid signature")
	ErrDuplicateTx          = errors.New("duplicate transaction")
	ErrInsufficientPrice    = errors.New("insufficient price")
	ErrInvalidType          = errors.New("invalid tx type")
	ErrInvalidID            = errors.New("invalid content ID")
	ErrInvalidSchema        = errors.New("invalid schema")
	ErrInvalidContent       = errors.New("invalid content")
	ErrContentAlreadyExists = errors.New("content already exists")
	ErrContentMissing       = errors.New("content does not exist")
	ErrWrongOwner           = errors.New("wrong owner")
	ErrInsufficientTip      = errors.New("insufficient tip")
	ErrAccountNotEmpty      = errors.New("account not empty")
	ErrServicerMissing      = errors.New("servicer missing")
	ErrTooManyTxs           = errors.New("too many transactions")
	ErrActionNotActivated   = errors.New("action not activated")
	ErrAuthNotActivated     = errors.New("auth not activated")
	ErrAuthFailed           = errors.New("auth failed")
	ErrMisalignedTime       = errors.New("misaligned time")
	ErrInvalidActor         = errors.New("invalid actor")
	ErrInvalidSponsor       = errors.New("invalid sponsor")

	// Execution Correctness
	ErrInvalidBalance  = errors.New("invalid balance")
	ErrBlockTooBig     = errors.New("block too big")
	ErrKeyNotSpecified = errors.New("key not specified")

	// Misc
	ErrNotImplemented         = errors.New("not implemented")
	ErrBlockNotProcessed      = errors.New("block is not processed")
	ErrInvalidKeyValue        = errors.New("invalid key or value")
	ErrModificationNotAllowed = errors.New("modification not allowed")
)

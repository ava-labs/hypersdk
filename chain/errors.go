// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
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
	ErrTimestampTooEarly           = errors.New("timestamp too early")
	ErrTimestampTooEarlyEmptyBlock = errors.New("timestamp too early for empty block")
	ErrTimestampTooLate            = errors.New("timestamp too late")
	ErrStateRootEmpty              = errors.New("state root empty")
	ErrNoTxs                       = errors.New("no transactions")
	ErrInvalidFee                  = errors.New("invalid fee")
	ErrInvalidUnitWindow           = errors.New("invalid unit window")
	ErrInvalidBlockCost            = errors.New("invalid block cost")
	ErrInvalidBlockWindow          = errors.New("invalid block window")
	ErrInvalidUnitsConsumed        = errors.New("invalid units consumed")
	ErrInsufficientSurplus         = errors.New("insufficient surplus fee")
	ErrInvalidSurplus              = errors.New("invalid surplus fee")
	ErrStateRootMismatch           = errors.New("state root mismatch")
	ErrInvalidResult               = errors.New("invalid result")
	ErrInvalidBlockHeight          = errors.New("invalid block height")

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
	ErrInvalidActor         = errors.New("invalid actor")
	ErrInvalidSponsor       = errors.New("invalid sponsor")
	ErrTooManyActions       = errors.New("too many actions")

	// Execution Correctness
	ErrBlockTooBig     = errors.New("block too big")
	ErrKeyNotSpecified = errors.New("key not specified")

	// State Correctness
	ErrFailedToFetchParentHeight    = errors.New("failed to fetch height from state")
	ErrFailedToFetchParentTimestamp = errors.New("failed to fetch timestamp from state")
	ErrFailedToFetchParentFee       = errors.New("failed to fetch fee manager from state")
	ErrFailedToParseParentHeight    = errors.New("failed to parse height")
	ErrFailedToParseParentTimestamp = errors.New("failed to parse timestamp")

	// Misc
	ErrNotImplemented         = errors.New("not implemented")
	ErrBlockNotProcessed      = errors.New("block is not processed")
	ErrInvalidKeyValue        = errors.New("invalid key or value")
	ErrModificationNotAllowed = errors.New("modification not allowed")
)

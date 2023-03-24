// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import "errors"

var (
	ErrInputEmpty          = errors.New("input is empty")
	ErrInvalidArgs         = errors.New("invalid args")
	ErrMissingSubcommand   = errors.New("must specify a subcommand")
	ErrIndexOutOfRange     = errors.New("index out-of-range")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInvalidChoice       = errors.New("invalid choice")
	ErrNotMultiple         = errors.New("must be a multiple")
	ErrInsufficientSupply  = errors.New("insufficient supply")
	ErrMustFill            = errors.New("must fill")
	ErrDuplicate           = errors.New("duplicate")
	ErrNoKeys              = errors.New("no available keys")
	ErrNoChains            = errors.New("no available chains")
	ErrTxFailed            = errors.New("tx failed")
)

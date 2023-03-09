package cmd

import "errors"

var (
	ErrInputEmpty          = errors.New("input is empty")
	ErrInvalidArgs         = errors.New("invalid args")
	ErrMissingSubcommand   = errors.New("must specify a subcommand")
	ErrIndexOutOfRange     = errors.New("index out-of-range")
	ErrInsufficientBalance = errors.New("insufficient balance")
	ErrInvalidChoice       = errors.New("invalid choice")
)

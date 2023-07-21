package cli

import "errors"

var (
	ErrInputEmpty          = errors.New("input is empty")
	ErrInvalidChoice       = errors.New("invalid choice")
	ErrIndexOutOfRange     = errors.New("index out-of-range")
	ErrInsufficientBalance = errors.New("insufficient balance")
)

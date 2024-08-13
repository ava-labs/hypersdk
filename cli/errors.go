package cli

import "errors"

var (
	ErrInputEmpty           = errors.New("input is empty")
	ErrInputTooLarge        = errors.New("input is too large")
	ErrInvalidChoice        = errors.New("invalid choice")
	ErrIndexOutOfRange      = errors.New("index out-of-range")
	ErrInsufficientBalance  = errors.New("insufficient balance")
	ErrDuplicate            = errors.New("duplicate")
	ErrNoChains             = errors.New("no available chains")
	ErrNoKeys               = errors.New("no available keys")
	ErrTxFailed             = errors.New("tx failed on-chain")
	ErrInsufficientAccounts = errors.New("insufficient accounts")
)

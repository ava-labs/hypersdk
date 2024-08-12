package cmd

import "errors"

var (
	ErrInvalidArgs       = errors.New("invalid args")
	ErrMissingSubcommand = errors.New("must specify a subcommand")
	ErrInvalidAddress    = errors.New("invalid address")
	ErrInvalidKeyType    = errors.New("invalid key type")
)

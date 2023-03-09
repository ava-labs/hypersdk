package cmd

import "errors"

var (
	ErrInvalidArgs       = errors.New("invalid args")
	ErrMissingSubcommand = errors.New("must specify a subcommand")
)

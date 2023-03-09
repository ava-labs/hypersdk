package cmd

import "errors"

var ErrMissingSubcommand = errors.New("must specify a subcommand")

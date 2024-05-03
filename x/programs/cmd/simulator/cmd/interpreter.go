package cmd

import (
	"context"

	"github.com/akamensky/argparse"
	"github.com/ava-labs/avalanchego/utils/logging"
)

var _ Cmd = &InterpreterCmd{}

type InterpreterCmd struct {
	cmd *argparse.Command
}

func (s InterpreterCmd) New(parser *argparse.Parser) Cmd {
	return InterpreterCmd{
		cmd: parser.NewCommand("interpreter", "Read input from a buffered stdin"),
	}
}

func (s InterpreterCmd) Run(ctx context.Context, log logging.Logger, args []string) error {
	return nil
}

func (s InterpreterCmd) Happened() bool {
	return s.cmd.Happened()
}

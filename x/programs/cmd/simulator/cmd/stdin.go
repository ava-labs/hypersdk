package cmd

import (
	"github.com/akamensky/argparse"
	"github.com/ava-labs/avalanchego/utils/logging"
)

var _ Cmd = &StdinCmd{}

type StdinCmd struct {
	cmd *argparse.Command
}

func (s StdinCmd) New(parser *argparse.Parser) Cmd {
	return StdinCmd{
		cmd: parser.NewCommand(s.Name(), "Read input from a buffered stdin"),
	}
}

func (s StdinCmd) Run(log logging.Logger) error {
	return nil
}

func (s StdinCmd) Happened() bool {
	return s.cmd.Happened()
}

func (s StdinCmd) Name() string {
	return "stdin"
}

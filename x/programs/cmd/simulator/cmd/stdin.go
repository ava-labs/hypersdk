package cmd

import (
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/spf13/cobra"
)

func newStdinCmd(log logging.Logger) *cobra.Command {
	return &cobra.Command{
		Use:   "stdin",
		Short: "Read input from a buffered stdin",
	}
}

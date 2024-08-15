// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/scripts"
	"github.com/ava-labs/hypersdk/utils"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Deploy an instance of MorpheusVM via the run.sh script",
	RunE: func(cmd *cobra.Command, args []string) error {
		output, err := scripts.RunScript("run")
		utils.Outf(string(output))
		return err
	},
}

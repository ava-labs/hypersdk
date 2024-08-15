// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/scripts"
	"github.com/ava-labs/hypersdk/utils"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stops an existing network created by `morpheuscli run`",
	RunE: func(cmd *cobra.Command, args []string) error {
		output, err := scripts.RunScript("stop")
		utils.Outf(string(output))
		return err
	},
}

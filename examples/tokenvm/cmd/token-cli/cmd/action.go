// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"github.com/spf13/cobra"
)

var actionCmd = &cobra.Command{
	Use: "action",
	RunE: func(*cobra.Command, []string) error {
		return ErrMissingSubcommand
	},
}

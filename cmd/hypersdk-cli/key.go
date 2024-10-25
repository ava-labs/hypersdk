// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"github.com/spf13/cobra"
)

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage keys",
}

func init() {
	rootCmd.AddCommand(keyCmd)
}

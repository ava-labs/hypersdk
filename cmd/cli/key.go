package main

import (
	"github.com/spf13/cobra"
)

var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Manage keys",
}

var keyBalanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Print current key balance",
	RunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

func init() {
	keyCmd.AddCommand(keyAddressCmd, keyBalanceCmd)
}

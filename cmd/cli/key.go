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

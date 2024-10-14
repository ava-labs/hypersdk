package main

import "github.com/spf13/cobra"

var endpointCmd = &cobra.Command{
	Use:   "endpoint",
	Short: "Manage endpoint",
}

func init() {
	rootCmd.AddCommand(endpointCmd)
}

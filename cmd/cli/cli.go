package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hypersdk-cli",
	Short: "HyperSDK CLI for interacting with HyperSDK-based chains",
	Long:  `A CLI application for performing read and write actions on HyperSDK-based chains.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}

func init() {
	rootCmd.PersistentFlags().StringP("output", "o", "human", "Output format (human or json)")
	rootCmd.PersistentFlags().String("endpoint", "", "Override the default endpoint")
	rootCmd.PersistentFlags().String("key", "", "Private ED25519 key as hex string")

	// Add key commands
	rootCmd.AddCommand(keyCmd)
}

func main() {
	Execute()
}

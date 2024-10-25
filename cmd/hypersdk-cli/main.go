// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

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
	rootCmd.PersistentFlags().StringP("output", "o", "text", "Output format (text or json)")
	rootCmd.PersistentFlags().String("endpoint", "", "Override the default endpoint")
	rootCmd.PersistentFlags().String("key", "", "Private ED25519 key as hex string")
}

func main() {
	Execute()
}

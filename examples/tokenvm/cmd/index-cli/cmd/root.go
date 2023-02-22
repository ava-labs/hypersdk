// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// "index-cli" implements indexvm client operation interface.
package cmd

import (
	"os"
	"time"

	"github.com/spf13/cobra"
)

const (
	requestTimeout = 30 * time.Second
	fsModeWrite    = 0o600
)

var (
	privateKeyFile string
	uri            string
	verbose        bool
	workDir        string

	rootCmd = &cobra.Command{
		Use:        "index-cli",
		Short:      "IndexVM CLI",
		SuggestFor: []string{"index-cli", "indexcli"},
	}
)

func init() {
	p, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	workDir = p

	cobra.EnablePrefixMatching = true
	rootCmd.AddCommand(
		createCmd,
		genesisCmd,
		transferCmd,
		networkCmd,
		watchCmd,
	)

	rootCmd.PersistentFlags().StringVar(
		&privateKeyFile,
		"private-key-file",
		".index-cli.pk",
		"private key file path",
	)
	rootCmd.PersistentFlags().StringVar(
		&uri,
		"endpoint",
		"",
		"RPC endpoint for VM",
	)
	rootCmd.PersistentFlags().BoolVar(
		&verbose,
		"verbose",
		false,
		"Print verbose information about operations",
	)
}

func Execute() error {
	return rootCmd.Execute()
}

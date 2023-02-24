// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// "token-cli" implements tokenvm client operation interface.
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
		Use:        "token-cli",
		Short:      "TokenVM CLI",
		SuggestFor: []string{"token-cli", "tokencli"},
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
		keyCmd,
		genesisCmd,
		transferCmd,
		networkCmd,
		watchCmd,
	)

	rootCmd.PersistentFlags().StringVar(
		&privateKeyFile,
		"private-key-file",
		".token-cli.pk",
		"private key file path",
	)
	rootCmd.PersistentFlags().StringVar(
		&uri,
		"endpoint",
		// We use the default local endpoint so we don't need to supply it in the
		// demo. If you change any of the contents of the genesis file, this will
		// change.
		"http://localhost:9650/ext/bc/2RbRfsZ7BX8kEwK7nJwWs2L8wGWTCkHfrAHVuHSjYyfwpyJ81n",
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

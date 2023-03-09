// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// "token-cli" implements tokenvm client operation interface.
package cmd

import (
	"os"
	"path/filepath"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/spf13/cobra"
)

const (
	requestTimeout = 30 * time.Second
	fsModeWrite    = 0o600
	databaseFolder = ".token-cli"
)

var (
	db      database.Database
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
	dbPath := filepath.Join(p, databaseFolder)
	db, err = pebble.New(dbPath, pebble.NewDefaultConfig())
	if err != nil {
		panic(err)
	}

	cobra.EnablePrefixMatching = true
	rootCmd.AddCommand(
		genesisCmd,
		// addChainCmd,
		// setChainCmd,
		// viewChainsCmd,

		keyCmd, // TODO: genKeyCmd,
		// importKeyCmd,
		// setKeyCmd,
		// viewKeysCmd,

		networkCmd, // TODO: call chainInfoCmd,
		watchCmd,   // TODO: watchChainCmd,

		balanceCmd,
		transferCmd,

		// importAssetCmd,
		// exportAssetCmd,

		createAssetCmd,
		mintAssetCmd,
		// burnAssetCmd,
		// modifyAssetCmd,

		createOrderCmd,
		fillOrderCmd,
		closeOrderCmd,

		// spamCmd,

		// resetCmd, (deletes DB)
	)
}

func Execute() error {
	return rootCmd.Execute()
}

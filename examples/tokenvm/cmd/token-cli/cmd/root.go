// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// "token-cli" implements tokenvm client operation interface.
package cmd

import (
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/hypersdk/pebble"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
)

const (
	requestTimeout  = 30 * time.Second
	fsModeWrite     = 0o600
	defaultDatabase = ".token-cli"
	defaultGenesis  = "genesis.json"
)

var (
	dbPath string
	db     database.Database

	genesisFile  string
	minUnitPrice int64

	rootCmd = &cobra.Command{
		Use:        "token-cli",
		Short:      "TokenVM CLI",
		SuggestFor: []string{"token-cli", "tokencli"},
	}
)

func init() {
	cobra.EnablePrefixMatching = true
	rootCmd.AddCommand(
		genesisCmd,
		keyCmd,
		chainCmd,
		actionCmd,
	)
	rootCmd.PersistentFlags().StringVar(
		&dbPath,
		"database",
		defaultDatabase,
		"path to database (will create it missing)",
	)
	rootCmd.PersistentPreRunE = func(*cobra.Command, []string) error {
		utils.Outf("{{yellow}}database:{{/}} %s\n", dbPath)
		var err error
		db, err = pebble.New(dbPath, pebble.NewDefaultConfig())
		return err
	}
	rootCmd.PersistentPostRunE = func(*cobra.Command, []string) error {
		return CloseDatabase()
	}
	rootCmd.SilenceErrors = true

	// genesis
	genesisCmd.AddCommand(
		genGenesisCmd,
	)
	genGenesisCmd.PersistentFlags().StringVar(
		&genesisFile,
		"genesis-file",
		defaultGenesis,
		"genesis file path",
	)
	genGenesisCmd.PersistentFlags().Int64Var(
		&minUnitPrice,
		"min-unit-price",
		-1,
		"minimum price",
	)

	// key
	keyCmd.AddCommand(
		genKeyCmd,
		importKeyCmd,
		setKeyCmd,
		balanceKeyCmd,
	)

	// chain
	chainCmd.AddCommand(
		importChainCmd,
		setChainCmd,
		chainInfoCmd,
		watchChainCmd,
	)

	// actions
	actionCmd.AddCommand(
		transferCmd,

		createAssetCmd,
		mintAssetCmd,
		// burnAssetCmd,
		// modifyAssetCmd,

		createOrderCmd,
		fillOrderCmd,
		closeOrderCmd,

		importAssetCmd,
		exportAssetCmd,
	)
}

func Execute() error {
	return rootCmd.Execute()
}

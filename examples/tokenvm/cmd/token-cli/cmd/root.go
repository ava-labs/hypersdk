// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// "token-cli" implements tokenvm client operation interface.
package cmd

import (
	"fmt"
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

	genesisFile        string
	minUnitPrice       int64
	maxBlockUnits      int64
	windowTargetUnits  int64
	windowTargetBlocks int64
	hideTxs            bool
	randomRecipient    bool
	maxTxBacklog       int
	deleteOtherChains  bool
	checkAllChains     bool
	prometheusFile     string
	prometheusData     string

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
		spamCmd,
		prometheusCmd,
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
	genGenesisCmd.PersistentFlags().Int64Var(
		&maxBlockUnits,
		"max-block-units",
		-1,
		"max block units",
	)
	genGenesisCmd.PersistentFlags().Int64Var(
		&windowTargetUnits,
		"window-target-units",
		-1,
		"window target units",
	)
	genGenesisCmd.PersistentFlags().Int64Var(
		&windowTargetBlocks,
		"window-target-blocks",
		-1,
		"window target blocks",
	)
	genesisCmd.AddCommand(
		genGenesisCmd,
	)

	// key
	balanceKeyCmd.PersistentFlags().BoolVar(
		&checkAllChains,
		"check-all-chains",
		false,
		"check all chains",
	)
	keyCmd.AddCommand(
		genKeyCmd,
		importKeyCmd,
		setKeyCmd,
		balanceKeyCmd,
	)

	// chain
	watchChainCmd.PersistentFlags().BoolVar(
		&hideTxs,
		"hide-txs",
		false,
		"hide txs",
	)
	importAvalancheOpsChainCmd.PersistentFlags().BoolVar(
		&deleteOtherChains,
		"delete-other-chains",
		true,
		"delete other chains",
	)
	chainCmd.AddCommand(
		importChainCmd,
		importANRChainCmd,
		importAvalancheOpsChainCmd,
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

	// spam
	runSpamCmd.PersistentFlags().BoolVar(
		&randomRecipient,
		"random-recipient",
		false,
		"random recipient",
	)
	runSpamCmd.PersistentFlags().IntVar(
		&maxTxBacklog,
		"max-tx-backlog",
		72_000,
		"max tx backlog",
	)
	spamCmd.AddCommand(
		runSpamCmd,
	)

	// prometheus
	generatePrometheusCmd.PersistentFlags().StringVar(
		&prometheusFile,
		"prometheus-file",
		"/tmp/prometheus.yaml",
		"prometheus file location",
	)
	generatePrometheusCmd.PersistentFlags().StringVar(
		&prometheusData,
		"prometheus-data",
		fmt.Sprintf("/tmp/prometheus-%d", time.Now().Unix()),
		"prometheus data location",
	)
	prometheusCmd.AddCommand(
		generatePrometheusCmd,
	)
}

func Execute() error {
	return rootCmd.Execute()
}

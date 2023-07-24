// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// "morpheus-cli" implements morpheusvm client operation interface.
package cmd

import (
	"fmt"
	"time"

	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/spf13/cobra"
)

const (
	fsModeWrite     = 0o600
	defaultDatabase = ".morpheus-cli"
	defaultGenesis  = "genesis.json"
)

var (
	handler *Handler

	dbPath            string
	genesisFile       string
	minUnitPrice      int64
	maxBlockUnits     int64
	windowTargetUnits int64
	minBlockGap       int64
	hideTxs           bool
	randomRecipient   bool
	maxTxBacklog      int
	checkAllChains    bool
	prometheusFile    string
	prometheusData    string

	rootCmd = &cobra.Command{
		Use:        "morpheus-cli",
		Short:      "BaseVM CLI",
		SuggestFor: []string{"morpheus-cli", "morpheuscli"},
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
		controller := NewController(dbPath)
		root, err := cli.New(controller)
		if err != nil {
			return err
		}
		handler = NewHandler(root)
		return err
	}
	rootCmd.PersistentPostRunE = func(*cobra.Command, []string) error {
		return handler.Root().CloseDatabase()
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
		&minBlockGap,
		"min-block-gap",
		-1,
		"minimum block gap (ms)",
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

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

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
	defaultDatabase = ".token-cli"
	defaultGenesis  = "genesis.json"
)

var (
	handler *Handler

	dbPath            string
	genesisFile       string
	minBlockGap       int64
	minUnitPrice      []string
	maxBlockUnits     []string
	windowTargetUnits []string
	hideTxs           bool
	randomRecipient   bool
	maxTxBacklog      int
	checkAllChains    bool
	prometheusFile    string
	prometheusData    string
	runPrometheus     bool
	maxFee            int64

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
		controller := NewController(dbPath)
		root, err := cli.New(controller)
		if err != nil {
			return err
		}
		handler = NewHandler(root)
		return nil
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
	genGenesisCmd.PersistentFlags().StringSliceVar(
		&minUnitPrice,
		"min-unit-price",
		[]string{},
		"minimum price",
	)
	genGenesisCmd.PersistentFlags().StringSliceVar(
		&maxBlockUnits,
		"max-block-units",
		[]string{},
		"max block units",
	)
	genGenesisCmd.PersistentFlags().StringSliceVar(
		&windowTargetUnits,
		"window-target-units",
		[]string{},
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
	runSpamCmd.PersistentFlags().Int64Var(
		&maxFee,
		"max-fee",
		-1,
		"max fee per tx",
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
	generatePrometheusCmd.PersistentFlags().BoolVar(
		&runPrometheus,
		"run-prometheus",
		true,
		"start prometheus",
	)
	prometheusCmd.AddCommand(
		generatePrometheusCmd,
	)
}

func Execute() error {
	return rootCmd.Execute()
}

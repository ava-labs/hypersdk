// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	fsModeWrite     = 0o600
	defaultDatabase = ".vmwithcontracts-cli"
	defaultGenesis  = "genesis.json"
)

var (
	handler *Handler

	dbPath                string
	genesisFile           string
	minUnitPrice          []string
	maxBlockUnits         []string
	windowTargetUnits     []string
	minBlockGap           int64
	hideTxs               bool
	checkAllChains        bool
	prometheusBaseURI     string
	prometheusOpenBrowser bool
	prometheusFile        string
	prometheusData        string
	startPrometheus       bool

	rootCmd = &cobra.Command{
		Use:        "vmwithcontracts-cli",
		Short:      "VmWithContracts CLI",
		SuggestFor: []string{"vmwithcontracts-cli", "vmwithcontractscli"},
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
		setChainCmd,
		chainInfoCmd,
		watchChainCmd,
	)

	// actions
	actionCmd.AddCommand(
		transferCmd,
		callCmd,
		publishFileCmd,
		deployCmd,
	)

	// spam
	spamCmd.AddCommand(
		runSpamCmd,
	)
}

func Execute() error {
	return rootCmd.Execute()
}

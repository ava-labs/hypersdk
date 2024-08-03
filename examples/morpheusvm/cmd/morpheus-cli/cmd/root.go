// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/cli"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	fsModeWrite     = 0o600
	defaultDatabase = ".morpheus-cli"
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
	accountsNumber        uint32
	sZipf                 float64
	vZipf                 float64
	txPerSec              uint32
	minCapacity           uint32
	txPerSecStep          uint32
	numClients            uint32

	rootCmd = &cobra.Command{
		Use:        "morpheus-cli",
		Short:      "MorpheusVM CLI",
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
	)

	// spam run ed25519 --accounts=10000000 --txs-per-second=100000 --min-capacity=15000 --step-size=1000 --s-zipf=1.0001 --v-zipf=2.7
	runSpamCmd.PersistentFlags().Uint32Var(
		&accountsNumber,
		"accounts",
		0,
		"number of accounts to create",
	)
	runSpamCmd.PersistentFlags().Float64Var(
		&sZipf,
		"s-zipf",
		0,
		"s-zipf",
	)
	runSpamCmd.PersistentFlags().Float64Var(
		&vZipf,
		"v-zipf",
		0,
		"v-zipf",
	)
	runSpamCmd.PersistentFlags().Uint32Var(
		&txPerSec,
		"txs-per-second",
		0,
		"txs per second",
	)
	runSpamCmd.PersistentFlags().Uint32Var(
		&txPerSecStep,
		"step-size",
		0,
		"step size",
	)
	runSpamCmd.PersistentFlags().Uint32Var(
		&numClients,
		"conns-per-host",
		0,
		"number of clients",
	)
	runSpamCmd.PersistentFlags().Uint32Var(
		&minCapacity,
		"min-capacity",
		0,
		"min capacity",
	)
	spamCmd.AddCommand(
		runSpamCmd,
	)

	// prometheus
	generatePrometheusCmd.PersistentFlags().StringVar(
		&prometheusBaseURI,
		"prometheus-base-uri",
		"http://localhost:9090",
		"prometheus server location",
	)
	generatePrometheusCmd.PersistentFlags().BoolVar(
		&prometheusOpenBrowser,
		"prometheus-open-browser",
		true,
		"open browser to prometheus dashboard",
	)
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
		&startPrometheus,
		"prometheus-start",
		true,
		"start local prometheus server",
	)
	prometheusCmd.AddCommand(
		generatePrometheusCmd,
	)
}

func Execute() error {
	return rootCmd.Execute()
}

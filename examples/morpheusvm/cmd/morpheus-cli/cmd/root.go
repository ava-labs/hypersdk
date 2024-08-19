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
	defaultDatabase = ".morpheus-cli"
	defaultGenesis  = "genesis.json"
)

var (
	handler *Handler

	dbPath                string
	genesisFile           string
	minUnitPrice          []string
	maxChunkUnits         []string
	minBlockGap           int64
	epochDuration         int64
	validityWindow        int64
	hideTxs               bool
	numAccounts           int
	txsPerSecond          int
	minCapacity           int
	stepSize              int
	sZipf                 float64
	vZipf                 float64
	plotZipf              bool
	connsPerHost          int
	clusterInfo           string
	privateKey            string
	checkAllChains        bool
	prometheusBaseURI     string
	prometheusOpenBrowser bool
	prometheusFile        string
	prometheusData        string
	startPrometheus       bool

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
		&maxChunkUnits,
		"max-chunk-units",
		[]string{},
		"max chunk units",
	)
	genGenesisCmd.PersistentFlags().Int64Var(
		&minBlockGap,
		"min-block-gap",
		-1,
		"minimum block gap (ms)",
	)
	genGenesisCmd.PersistentFlags().Int64Var(
		&epochDuration,
		"epoch-duration",
		-1,
		"epoch duration (ms)",
	)
	genGenesisCmd.PersistentFlags().Int64Var(
		&validityWindow,
		"validity-window",
		-1,
		"validity window (ms)",
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
		importAvalancheCliChainCmd,
		setChainCmd,
		chainInfoCmd,
		watchChainCmd,
		watchPreConfsCmd,
		watchPreConfsStandCmd,
		anchorsCmd,
	)

	// actions
	actionCmd.AddCommand(
		transferCmd,
		anchorCmd,
	)

	// spam
	runSpamCmd.PersistentFlags().Float64Var(
		&sZipf,
		"s-zipf",
		1.01,
		"Zipf distribution = [(v+k)^(-s)]",
	)
	runSpamCmd.PersistentFlags().Float64Var(
		&vZipf,
		"v-zipf",
		2.7,
		"Zipf distribution = [(v+k)^(-s)]",
	)
	runSpamCmd.PersistentFlags().BoolVar(
		&plotZipf,
		"plot-zipf",
		false,
		"plot zipf distribution",
	)
	runSpamCmd.PersistentFlags().IntVar(
		&numAccounts,
		"accounts",
		-1,
		"number of accounts submitting txs",
	)
	runSpamCmd.PersistentFlags().IntVar(
		&txsPerSecond,
		"txs-per-second",
		-1,
		"number of txs issued per second (under backlog)",
	)
	runSpamCmd.PersistentFlags().IntVar(
		&minCapacity,
		"min-capacity",
		-1,
		"minimum txs per second chain can handle",
	)
	runSpamCmd.PersistentFlags().IntVar(
		&stepSize,
		"step-size",
		-1,
		"amount to increase TPS target",
	)
	runSpamCmd.PersistentFlags().IntVar(
		&connsPerHost,
		"conns-per-host",
		-1,
		"number of connections to create per host",
	)
	runSpamCmd.PersistentFlags().StringVar(
		&clusterInfo,
		"cluster-info",
		"",
		"output from avalanche-cli with cluster info",
	)
	runSpamCmd.PersistentFlags().StringVar(
		&privateKey,
		"private-key",
		"",
		"ed25519 private key for root account (hex)",
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

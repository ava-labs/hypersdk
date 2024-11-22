// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests/e2e"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/tests/workload"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	owner      = "morpheus-cli"
	devNetPort = "9560"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Quickly deploy an instance of MorpheusVM",
	RunE: func(cmd *cobra.Command, args []string) error {
		genesis, _, err := workload.New(minBlockGap)
		if err != nil {
			return err
		}
		genesisBytes, err := json.Marshal(genesis)
		if err != nil {
			return err
		}

		network, err := e2e.NewMorpheusNetwork(numOfNodes, devNetPort, genesisBytes, owner, avalancheGoPath, avalancheGoPluginDir)
		if err != nil {
			return err
		}

		utils.Outf("\nBootstrapped Network")
		var rpcURL strings.Builder
		if _, err := rpcURL.WriteString("http://127.0.0.1:"); err != nil {
			return err
		}
		if _, err := rpcURL.WriteString(devNetPort); err != nil {
			return err
		}
		if _, err := rpcURL.WriteString("/ext/bc/"); err != nil {
			return err
		}
		if _, err := rpcURL.WriteString(network.Subnets[0].Chains[0].ChainID.String()); err != nil {
			return err
		}
		if _, err := rpcURL.WriteString(rpc.JSONRPCEndpoint); err != nil {
			return err
		}
		utils.Outf("\nRPC URL is: %v\n", rpcURL.String())

		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

		<-signals
		if err := network.Stop(context.Background()); err != nil {
			panic(err)
		}
		utils.Outf("\nClosed network\n")

		return nil
	},
}

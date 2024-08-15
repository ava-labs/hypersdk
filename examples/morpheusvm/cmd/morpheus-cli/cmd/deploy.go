// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package cmd

import (
	"context"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/onsi/ginkgo/v2"
	"github.com/spf13/cobra"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/tests/fixture"
	"github.com/ava-labs/hypersdk/utils"

	le2e "github.com/ava-labs/hypersdk/examples/morpheusvm/tests/e2e"
)

const owner = "morpheus-cli"

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Quickly deploy an instance of MorpheusVM",
	RunE: func(cmd *cobra.Command, args []string) error {
		genesisBytes, err := le2e.DefaultGenesisValues()
		if err != nil {
			return err
		}

		nodes := tmpnet.NewNodesOrPanic(numOfNodes)
		subnet := fixture.NewHyperVMSubnet(
			consts.Name,
			consts.ID,
			genesisBytes,
			nodes...,
		)

		timeOut := 2 * time.Minute

		ctx, cancel := context.WithTimeout(context.Background(), timeOut)
		defer cancel()

		network := fixture.NewTmpnetNetwork(owner, nodes, subnet)
		if err := tmpnet.BootstrapNewNetwork(
			ctx,
			ginkgo.GinkgoWriter,
			network,
			"",
			avalancheGoPath,
			avalancheGoPluginDir,
		); err != nil {
			utils.Outf(err.Error())
			return err
		}

		utils.Outf("\nBootstrapped Network")
		var rpc_url strings.Builder
		rpc_url.WriteString(nodes[0].URI)
		rpc_url.WriteString("/ext/bc/")
		rpc_url.WriteString(subnet.Chains[0].ChainID.String())
		rpc_url.WriteString(rpc.JSONRPCEndpoint)

		utils.Outf("\nRPC URL is: %v\n", rpc_url.String())

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

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ava-labs/avalanchego/api/admin"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/examples/evm/consts"
	"github.com/ava-labs/hypersdk/examples/evm/genesis"
	"github.com/ava-labs/hypersdk/examples/evm/storage"
	"github.com/ava-labs/hypersdk/tests/fixture"
)

const nodeCount = 5

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	avalancheGoExecPath := os.Getenv("AVALANCHEGO_EXEC_PATH")
	if avalancheGoExecPath == "" {
		panic("AVALANCHEGO_EXEC_PATH not set")
	}
	avalancheGoPluginPath := os.Getenv("AVALANCHEGO_PLUGIN_PATH")
	if avalancheGoPluginPath == "" {
		panic("AVALANCHEGO_PLUGIN_PATH not set")
	}

	log := logging.NewLogger(
		"hypersdk",
		logging.NewWrappedCore(
			logging.Info,
			os.Stdout,
			logging.Colors.ConsoleEncoder(),
		),
	)

	// Run only once in the first ginkgo process
	nodes := tmpnet.NewNodesOrPanic(nodeCount)
	nodes[0].Flags[config.HTTPPortKey] = config.DefaultHTTPPort

	genesis, err := genesis.New((100 * time.Millisecond))
	if err != nil {
		panic(err)
	}

	prefundedAddresses := make([]common.Address, 0)
	for _, alloc := range genesis.CustomAllocation {
		prefundedAddresses = append(prefundedAddresses, storage.ToEVMAddress(alloc.Address))
	}
	log.Info("genesis info", zap.Stringers("prefunded EVM addresses", prefundedAddresses))

	genesisBytes, err := json.Marshal(genesis)
	if err != nil {
		panic(err)
	}
	subnet := fixture.NewHyperVMSubnet(
		consts.Name,
		consts.ID,
		genesisBytes,
		nodes...,
	)

	network := fixture.NewTmpnetNetwork(
		"hyperevm",
		nodes,
		subnet,
	)

	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(3*time.Minute))
	defer cancel()

	err = tmpnet.BootstrapNewNetwork(
		ctx,
		log,
		network,
		"",
		avalancheGoExecPath,
		avalancheGoPluginPath,
	)
	if err != nil {
		panic(err)
	}

	adminClient := admin.NewClient("http://localhost:9650")
	if err := adminClient.AliasChain(context.Background(), subnet.Chains[0].ChainID.String(), consts.Name); err != nil {
		network.Stop(context.Background())
		panic(err)
	}

	uris := network.GetNodeURIs()
	log.Info("main Node Info", zap.String("uris", uris[0].URI))

	<-c
	if err := network.Stop(context.Background()); err != nil {
		log.Error("failed to stop network gracefully", zap.Error(err))
	}
	log.Info("exiting")
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package exec

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/onsi/ginkgo/v2"

	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/tests/fixture"
)

const (
	owner = "deployer"
)

func Deploy() {
	avalancheGoPath := flag.String("avalanchego-path", "", "path to avalanchego binary")
	avalancheGoPluginDir := flag.String("plugin-dir", "", "path for plugin binaries")
	numOfNodes := flag.Int("num-of-nodes", 0, "number of nodes to include in the network")

	flag.Parse()

	prefundedAddrStr := "morpheus1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdjk97rwu"

	gen := genesis.Default()
	// Set WindowTargetUnits to MaxUint64 for all dimensions to iterate full mempool during block building.
	gen.WindowTargetUnits = fees.Dimensions{math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	// Set all lmiits to MaxUint64 to avoid limiting block size for all dimensions except bandwidth. Must limit bandwidth to avoid building
	// a block that exceeds the maximum size allowed by AvalancheGo.
	gen.MaxBlockUnits = fees.Dimensions{1800000, math.MaxUint64, math.MaxUint64, math.MaxUint64, math.MaxUint64}
	gen.MinBlockGap = 100
	gen.CustomAllocation = []*genesis.CustomAllocation{
		{
			Address: prefundedAddrStr,
			Balance: 10_000_000_000_000,
		},
	}
	genesisBytes, err := json.Marshal(gen)
	if err != nil {
		log.Println("failed to marshal genesis")
		return
	}

	nodes := tmpnet.NewNodesOrPanic(*numOfNodes)
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
		*avalancheGoPath,
		*avalancheGoPluginDir,
	); err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Bootstrapped Network!")
	fmt.Println(network.GetNodeURIs())

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	<-signals
	if err := network.Stop(context.Background()); err != nil {
		panic(err)
	}
	fmt.Println("\nClosed network")
}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"context"
	"fmt"
	"time"

	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/stretchr/testify/require"

	runner_sdk "github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/tests/workload"
	"github.com/ava-labs/hypersdk/utils"
)

const (
	modeTest = "test"
	modeRun  = "run"
)

var (
	networkConfig     Config
	network           *embeddedVMNetwork
	uris              []string
	_                 workload.Network = (*embeddedVMNetwork)(nil)
	txWorkloadFactory workload.TxWorkloadFactory
	anrCli            runner_sdk.Client
	trackSubnetsOpt   runner_sdk.OpOption
	mode              string
	blockchainID      string
)

func SetWorkloadFactory(
	factory workload.TxWorkloadFactory,
) {
	txWorkloadFactory = factory
}

type embeddedVMNetwork struct {
	uris      []string
	networkID uint32
	chainID   ids.ID
}

func (e *embeddedVMNetwork) URIs() []string {
	return e.uris
}

func (e *embeddedVMNetwork) Description() workload.Description {
	return workload.Description{
		NetworkID: e.networkID,
		ChainID:   e.chainID,
	}
}

type Config struct {
	Name                       string
	Mode                       string
	ExecPath                   string
	NumValidators              uint
	GRPCEp                     string
	NetworkRunnerLogLevel      string
	AvalanchegoLogLevel        string
	AvalanchegoLogDisplayLevel string
	VMGenesisPath              string
	VMConfigPath               string
	SubnetConfigPath           string
	PluginDir                  string
}

func CreateE2ENetwork(
	ctx context.Context,
	c Config,

) {
	networkConfig = c
	require := require.New(ginkgo.GinkgoT())
	require.Contains([]string{modeTest, modeRun}, networkConfig.Mode)
	require.Greater(networkConfig.NumValidators, uint(0))

	logLevel, err := logging.ToLevel(networkConfig.NetworkRunnerLogLevel)
	require.NoError(err)
	logFactory := logging.NewFactory(logging.Config{
		DisplayLevel: logLevel,
		LogLevel:     logLevel,
	})
	log, err := logFactory.Make("main")
	require.NoError(err)

	anrCli, err = runner_sdk.New(runner_sdk.Config{
		Endpoint:    networkConfig.GRPCEp,
		DialTimeout: 10 * time.Second,
	}, log)
	require.NoError(err)

	// Start cluster
	startClusterCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	resp, err := anrCli.Start(
		startClusterCtx,
		networkConfig.ExecPath,
		runner_sdk.WithPluginDir(networkConfig.PluginDir),
		runner_sdk.WithGlobalNodeConfig(fmt.Sprintf(`{
				"log-level":"%s",
				"log-display-level":"%s",
				"proposervm-use-current-height":true,
				"throttler-inbound-validator-alloc-size":"10737418240",
				"throttler-inbound-at-large-alloc-size":"10737418240",
				"throttler-inbound-node-max-at-large-bytes":"10737418240",
				"throttler-inbound-node-max-processing-msgs":"1000000",
				"throttler-inbound-bandwidth-refill-rate":"1073741824",
				"throttler-inbound-bandwidth-max-burst-size":"1073741824",
				"throttler-inbound-cpu-validator-alloc":"100000",
				"throttler-inbound-cpu-max-non-validator-usage":"100000",
				"throttler-inbound-cpu-max-non-validator-node-usage":"100000",
				"throttler-inbound-disk-validator-alloc":"10737418240000",
				"throttler-outbound-validator-alloc-size":"10737418240",
				"throttler-outbound-at-large-alloc-size":"10737418240",
				"throttler-outbound-node-max-at-large-bytes": "10737418240",
				"network-compression-type":"zstd",
				"consensus-app-concurrency":"512",
				"profile-continuous-enabled":true,
				"profile-continuous-freq":"1m",
				"http-host":"",
				"http-allowed-origins": "*",
				"http-allowed-hosts": "*"
			}`,
			networkConfig.AvalanchegoLogLevel,
			networkConfig.AvalanchegoLogDisplayLevel,
		)),
	)
	cancel()
	require.NoError(err)
	utils.Outf(
		"{{green}}successfully started cluster:{{/}} %s {{green}}subnets:{{/}} %+v\n",
		resp.ClusterInfo.RootDataDir,
		resp.GetClusterInfo().GetSubnets(),
	)
	logsDir := resp.GetClusterInfo().GetRootDataDir()

	// Add numValidators (already have BLS key registered)
	subnet := []string{}
	for i := 1; i <= int(networkConfig.NumValidators); i++ {
		n := fmt.Sprintf("node%d", i)
		subnet = append(subnet, n)
	}
	specs := []*rpcpb.BlockchainSpec{
		{
			VmName:      networkConfig.Name,
			Genesis:     networkConfig.VMGenesisPath,
			ChainConfig: networkConfig.VMConfigPath,
			SubnetSpec: &rpcpb.SubnetSpec{
				SubnetConfig: networkConfig.SubnetConfigPath,
				Participants: subnet,
			},
		},
	}

	// Create subnet
	createBlockchainsCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	sresp, err := anrCli.CreateBlockchains(
		createBlockchainsCtx,
		specs,
	)
	cancel()
	require.NoError(err)

	blockchainID = sresp.ChainIds[0]
	networkID := sresp.ClusterInfo.NetworkId
	subnetID := sresp.ClusterInfo.CustomChains[blockchainID].SubnetId
	utils.Outf(
		"{{green}}successfully added chain:{{/}} %s {{green}}subnet:{{/}} %s {{green}}participants:{{/}} %+v\n",
		blockchainID,
		subnetID,
		subnet,
	)
	trackSubnetsOpt = runner_sdk.WithGlobalNodeConfig(fmt.Sprintf(`{"%s":"%s"}`,
		config.TrackSubnetsKey,
		subnetID,
	))

	require.NotEmpty(blockchainID)
	require.NotEmpty(logsDir)

	cctx, ccancel := context.WithTimeout(ctx, 2*time.Minute)
	status, err := anrCli.Status(cctx)
	defer ccancel()
	require.NoError(err)
	nodeInfos := status.GetClusterInfo().GetNodeInfos()

	for _, nodeName := range subnet {
		info := nodeInfos[nodeName]
		u := fmt.Sprintf("%s/ext/bc/%s", info.Uri, blockchainID)
		cli := rpc.NewJSONRPCClient(u)

		// After returning healthy, the node may not respond right away
		//
		// TODO: figure out why
		for i := 0; i < 10; i++ {
			_, _, _, err = cli.Network(cctx)
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}
		}
		require.NoError(err)

		uris = append(uris, u)
	}

	network = &embeddedVMNetwork{
		uris:      uris,
		networkID: networkID,
		chainID:   ids.FromStringOrPanic(blockchainID),
	}
}

var _ = ginkgo.Describe("[HyperSDK APIs]", func() {
	ctx := context.Background()
	require := require.New(ginkgo.GinkgoT())

	ginkgo.It("Ping", func() {
		workload.Ping(ctx, require, network)
	})

	ginkgo.It("GetNetwork", func() {
		workload.GetNetwork(ctx, require, network)
	})

	ginkgo.It("BasicTxWorkload", func() {
		txs, err := txWorkloadFactory.NewBasicTxWorkload(uris[0])
		require.NoError(err)
		workload.ExecuteWorkload(ctx, require, network, txs)
	})

	ginkgo.It("Syncing", ginkgo.Serial, func() {
		ginkgo.By("Generate 128 blocks", func() {
			workload.GenerateNBlocks(ctx, require, network, txWorkloadFactory, 128)
		})

		var bootstrapNodeURI string
		ginkgo.By("Start a new node to bootstrap", func() {
			cluster, err := anrCli.AddNode(
				ctx,
				"bootstrap",
				networkConfig.ExecPath,
				trackSubnetsOpt,
				runner_sdk.WithChainConfigs(map[string]string{
					blockchainID: networkConfig.VMConfigPath,
				}),
			)
			require.NoError(err)
			awaitHealthy(anrCli)

			nodeURI := cluster.ClusterInfo.NodeInfos["bootstrap"].Uri
			uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainID)
			utils.Outf("{{blue}}bootstrap node uri: %s{{/}}\n", uri)
			bootstrapNodeURI = uri
			network.uris = append(network.uris, bootstrapNodeURI)
			require.NoError(err)
		})
		ginkgo.By("Accept a transaction after state sync", func() {
			txWorkload, err := txWorkloadFactory.NewSizedTxWorkload(bootstrapNodeURI, 1)
			require.NoError(err)
			workload.ExecuteWorkload(ctx, require, network, txWorkload)
		})
		ginkgo.By("Restart the node", func() {
			cluster, err := anrCli.RestartNode(context.Background(), "bootstrap")
			require.NoError(err)

			// Upon restart, the node should be able to read blocks from disk
			// to initialize its [seen] index and become ready in less than
			// [ValidityWindow].
			start := time.Now()
			awaitHealthy(anrCli)
			require.WithinDuration(time.Now(), start, 20*time.Second)

			// Update bootstrap info to latest in case it was assigned a new port
			nodeURI := cluster.ClusterInfo.NodeInfos["bootstrap"].Uri
			uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainID)
			require.NoError(err)
			utils.Outf("{{blue}}bootstrap node uri: %s{{/}}\n", uri)
			c := rpc.NewJSONRPCClient(uri)
			_, _, _, err = c.Network(ctx)
			require.NoError(err)
			bootstrapNodeURI = uri
			network.uris[len(network.uris)-1] = bootstrapNodeURI
		})
		ginkgo.By("Generate 1024 blocks", func() {
			workload.GenerateNBlocks(ctx, require, network, txWorkloadFactory, 1024)
		})
		var syncNodeURI string
		ginkgo.By("Start a new node to state sync", func() {
			cluster, err := anrCli.AddNode(
				ctx,
				"sync",
				networkConfig.ExecPath,
				trackSubnetsOpt,
				runner_sdk.WithChainConfigs(map[string]string{
					blockchainID: networkConfig.VMConfigPath,
				}),
			)
			require.NoError(err)

			awaitHealthy(anrCli)

			nodeURI := cluster.ClusterInfo.NodeInfos["sync"].Uri
			uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainID)
			require.NoError(err)
			utils.Outf("{{blue}}sync node uri: %s{{/}}\n", uri)
			c := rpc.NewJSONRPCClient(uri)
			_, _, _, err = c.Network(ctx)
			require.NoError(err)
			syncNodeURI = uri
		})
		ginkgo.By("Accept a transaction after state sync", func() {
			txWorkload, err := txWorkloadFactory.NewSizedTxWorkload(syncNodeURI, 1)
			require.NoError(err)
			workload.ExecuteWorkload(ctx, require, network, txWorkload)
		})
		ginkgo.By("Pause the node", func() {
			// shuts down the node and keeps all db/conf data for a proper restart
			_, err := anrCli.PauseNode(
				ctx,
				"sync",
			)
			require.NoError(err)

			awaitHealthy(anrCli)

			c := rpc.NewJSONRPCClient(syncNodeURI)
			ok, err := c.Ping(ctx)
			require.Error(err)
			require.False(ok)
		})
		ginkgo.By("Generate 256 blocks", func() {
			workload.GenerateNBlocks(ctx, require, network, txWorkloadFactory, 256)
		})
		ginkgo.By("Resume the node", func() {
			cluster, err := anrCli.ResumeNode(
				context.Background(),
				"sync",
			)
			require.NoError(err)

			awaitHealthy(anrCli)

			// Update sync info to latest in case it was assigned a new port
			nodeURI := cluster.ClusterInfo.NodeInfos["sync"].Uri
			uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainID)
			require.NoError(err)
			utils.Outf("{{blue}}sync node uri: %s{{/}}\n", uri)
			c := rpc.NewJSONRPCClient(uri)
			_, _, _, err = c.Network(ctx)
			require.NoError(err)
			syncNodeURI = uri
		})

		ginkgo.By("Accept a transaction after resuming", func() {
			txWorkload, err := txWorkloadFactory.NewSizedTxWorkload(syncNodeURI, 1)
			require.NoError(err)
			workload.ExecuteWorkload(ctx, require, network, txWorkload)
		})
		ginkgo.By("State sync while broadcasting txs", func() {
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				// Recover failure if exits
				defer ginkgo.GinkgoRecover()

				workload.GenerateNBlocks(ctx, require, network, txWorkloadFactory, 128)
			}()

			// Give time for transactions to start processing
			time.Sleep(5 * time.Second)

			// Start syncing node
			cluster, err := anrCli.AddNode(
				ctx,
				"sync_concurrent",
				networkConfig.ExecPath,
				trackSubnetsOpt,
				runner_sdk.WithChainConfigs(map[string]string{
					blockchainID: networkConfig.VMConfigPath,
				}),
			)
			require.NoError(err)
			awaitHealthy(anrCli)

			nodeURI := cluster.ClusterInfo.NodeInfos["sync_concurrent"].Uri
			uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainID)
			utils.Outf("{{blue}}sync node uri: %s{{/}}\n", uri)
			c := rpc.NewJSONRPCClient(uri)
			_, _, _, err = c.Network(ctx)
			require.NoError(err)
			cancel()
		})
		ginkgo.By("Accept a transaction after syncing", func() {
			txWorkload, err := txWorkloadFactory.NewSizedTxWorkload(uris[0], 1)
			require.NoError(err)
			workload.ExecuteWorkload(ctx, require, network, txWorkload)
		})
	})
})

var _ = ginkgo.AfterSuite(func() {
	require := require.New(ginkgo.GinkgoT())

	switch mode {
	case modeTest:
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_, err := anrCli.Stop(ctx)
		require.NoError(err)

	case modeRun:
		utils.Outf("{{yellow}}skipping cluster shutdown{{/}}\n\n")
		utils.Outf("{{cyan}}Blockchain:{{/}} %s\n", blockchainID)
		for i, uri := range uris {
			utils.Outf("Node %d URI: %s\n", i, uri)
		}
	}
	require.NoError(anrCli.Close())
})

func awaitHealthy(cli runner_sdk.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	rpc.Wait(ctx, func(ctx context.Context) (bool, error) {
		_, err := cli.Health(ctx)
		return err == nil, nil
	})
}

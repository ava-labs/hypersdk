// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/fatih/color"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/rpc"

	runner_sdk "github.com/ava-labs/avalanche-network-runner/client"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	hutils "github.com/ava-labs/hypersdk/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
)

const (
	startAmount = uint64(10000000000000000000)
	sendAmount  = uint64(5000)

	healthPollInterval = 10 * time.Second
)

func TestE2e(t *testing.T) {
	ginkgo.RunSpecs(t, "tokenvm e2e test suites")
}

var (
	requestTimeout time.Duration

	networkRunnerLogLevel      string
	avalanchegoLogLevel        string
	avalanchegoLogDisplayLevel string
	gRPCEp                     string
	gRPCGatewayEp              string

	execPath  string
	pluginDir string

	vmGenesisPath    string
	vmConfigPath     string
	vmConfig         string
	subnetConfigPath string
	outputPath       string

	mode string

	logsDir string

	blockchainID string

	trackSubnetsOpt runner_sdk.OpOption

	numValidators uint
)

func init() {
	flag.DurationVar(
		&requestTimeout,
		"request-timeout",
		120*time.Second,
		"timeout for transaction issuance and confirmation",
	)

	flag.StringVar(
		&networkRunnerLogLevel,
		"network-runner-log-level",
		"info",
		"gRPC server endpoint",
	)

	flag.StringVar(
		&avalanchegoLogLevel,
		"avalanchego-log-level",
		"info",
		"log level for avalanchego",
	)

	flag.StringVar(
		&avalanchegoLogDisplayLevel,
		"avalanchego-log-display-level",
		"error",
		"log display level for avalanchego",
	)

	flag.StringVar(
		&gRPCEp,
		"network-runner-grpc-endpoint",
		"0.0.0.0:8080",
		"gRPC server endpoint",
	)
	flag.StringVar(
		&gRPCGatewayEp,
		"network-runner-grpc-gateway-endpoint",
		"0.0.0.0:8081",
		"gRPC gateway endpoint",
	)

	flag.StringVar(
		&execPath,
		"avalanchego-path",
		"",
		"avalanchego executable path",
	)

	flag.StringVar(
		&pluginDir,
		"avalanchego-plugin-dir",
		"",
		"avalanchego plugin directory",
	)

	flag.StringVar(
		&vmGenesisPath,
		"vm-genesis-path",
		"",
		"VM genesis file path",
	)

	flag.StringVar(
		&vmConfigPath,
		"vm-config-path",
		"",
		"VM configfile path",
	)

	flag.StringVar(
		&subnetConfigPath,
		"subnet-config-path",
		"",
		"Subnet configfile path",
	)

	flag.StringVar(
		&outputPath,
		"output-path",
		"",
		"output YAML path to write local cluster information",
	)

	flag.StringVar(
		&mode,
		"mode",
		"test",
		"'test' to shut down cluster after tests, 'run' to skip tests and only run without shutdown",
	)

	flag.UintVar(
		&numValidators,
		"num-validators",
		5,
		"number of validators per blockchain",
	)
}

const (
	modeTest     = "test"
	modeFullTest = "full-test" // runs state sync
	modeRun      = "run"
)

var anrCli runner_sdk.Client

var _ = ginkgo.BeforeSuite(func() {
	require := require.New(ginkgo.GinkgoT())

	require.Contains([]string{modeTest, modeFullTest, modeRun}, mode)
	require.Greater(numValidators, uint(0))
	logLevel, err := logging.ToLevel(networkRunnerLogLevel)
	require.NoError(err)
	logFactory := logging.NewFactory(logging.Config{
		DisplayLevel: logLevel,
		LogLevel:     logLevel,
	})
	log, err := logFactory.Make("main")
	require.NoError(err)

	anrCli, err = runner_sdk.New(runner_sdk.Config{
		Endpoint:    gRPCEp,
		DialTimeout: 10 * time.Second,
	}, log)
	require.NoError(err)

	hutils.Outf(
		"{{green}}sending 'start' with binary path:{{/}} %q (%q)\n",
		execPath,
		consts.ID,
	)

	// Load config data
	if len(vmConfigPath) > 0 {
		configData, err := os.ReadFile(vmConfigPath)
		require.NoError(err)
		vmConfig = string(configData)
	} else {
		vmConfig = "{}"
	}

	// Start cluster
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	resp, err := anrCli.Start(
		ctx,
		execPath,
		runner_sdk.WithPluginDir(pluginDir),
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
			avalanchegoLogLevel,
			avalanchegoLogDisplayLevel,
		)),
	)
	cancel()
	require.NoError(err)
	hutils.Outf(
		"{{green}}successfully started cluster:{{/}} %s {{green}}subnets:{{/}} %+v\n",
		resp.ClusterInfo.RootDataDir,
		resp.GetClusterInfo().GetSubnets(),
	)
	logsDir = resp.GetClusterInfo().GetRootDataDir()

	// Add 5 validators (already have BLS key registered)
	subnet := []string{}
	for i := 1; i <= int(numValidators); i++ {
		n := fmt.Sprintf("node%d", i)
		subnet = append(subnet, n)
	}
	specs := []*rpcpb.BlockchainSpec{
		{
			VmName:      consts.Name,
			Genesis:     vmGenesisPath,
			ChainConfig: vmConfigPath,
			SubnetSpec: &rpcpb.SubnetSpec{
				SubnetConfig: subnetConfigPath,
				Participants: subnet,
			},
		},
	}

	// Create subnet
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
	sresp, err := anrCli.CreateBlockchains(
		ctx,
		specs,
	)
	cancel()
	require.NoError(err)

	blockchainID = sresp.ChainIds[0]
	subnetID := sresp.ClusterInfo.CustomChains[blockchainID].SubnetId
	hutils.Outf(
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

	cctx, ccancel := context.WithTimeout(context.Background(), 2*time.Minute)
	status, err := anrCli.Status(cctx)
	ccancel()
	require.NoError(err)
	nodeInfos := status.GetClusterInfo().GetNodeInfos()

	instances = []instance{}
	for _, nodeName := range subnet {
		info := nodeInfos[nodeName]
		u := fmt.Sprintf("%s/ext/bc/%s", info.Uri, blockchainID)
		bid, err := ids.FromString(blockchainID)
		require.NoError(err)
		nodeID, err := ids.NodeIDFromString(info.GetId())
		require.NoError(err)
		cli := rpc.NewJSONRPCClient(u)

		// After returning healthy, the node may not respond right away
		//
		// TODO: figure out why
		var networkID uint32
		for i := 0; i < 10; i++ {
			networkID, _, _, err = cli.Network(context.TODO())
			if err != nil {
				time.Sleep(1 * time.Second)
				continue
			}
		}
		require.NoError(err)

		instances = append(instances, instance{
			nodeID: nodeID,
			uri:    u,
			cli:    cli,
			tcli:   trpc.NewJSONRPCClient(u, networkID, bid),
		})
	}

	// Load default pk
	privBytes, err := codec.LoadHex(
		"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
		ed25519.PrivateKeyLen,
	)
	require.NoError(err)
	priv = ed25519.PrivateKey(privBytes)
	factory = auth.NewED25519Factory(priv)
	rsender = auth.NewED25519Address(priv.PublicKey())
	sender = codec.MustAddressBech32(consts.HRP, rsender)
	hutils.Outf("\n{{yellow}}$ loaded address:{{/}} %s\n\n", sender)
})

var (
	priv    ed25519.PrivateKey
	factory *auth.ED25519Factory
	rsender codec.Address
	sender  string

	instances []instance
)

type instance struct {
	nodeID ids.NodeID
	uri    string
	cli    *rpc.JSONRPCClient
	tcli   *trpc.JSONRPCClient
}

var _ = ginkgo.AfterSuite(func() {
	require := require.New(ginkgo.GinkgoT())

	switch mode {
	case modeTest, modeFullTest:
		hutils.Outf("{{red}}shutting down cluster{{/}}\n")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		_, err := anrCli.Stop(ctx)
		cancel()
		require.NoError(err)

	case modeRun:
		hutils.Outf("{{yellow}}skipping cluster shutdown{{/}}\n\n")
		hutils.Outf("{{cyan}}Blockchain:{{/}} %s\n", blockchainID)
		for _, member := range instances {
			hutils.Outf("%s URI: %s\n", member.nodeID, member.uri)
		}
	}
	require.NoError(anrCli.Close())
})

var _ = ginkgo.Describe("[Ping]", func() {
	require := require.New(ginkgo.GinkgoT())

	ginkgo.It("can ping", func() {
		for _, inst := range instances {
			cli := inst.cli
			ok, err := cli.Ping(context.Background())
			require.NoError(err)
			require.True(ok)
		}
	})
})

var _ = ginkgo.Describe("[Network]", func() {
	require := require.New(ginkgo.GinkgoT())

	ginkgo.It("can get network", func() {
		for _, inst := range instances {
			cli := inst.cli
			_, _, chainID, err := cli.Network(context.Background())
			require.NoError(err)
			require.NotEqual(chainID, ids.Empty)
		}
	})
})

var _ = ginkgo.Describe("[Test]", func() {
	require := require.New(ginkgo.GinkgoT())

	switch mode {
	case modeRun:
		hutils.Outf("{{yellow}}skipping tests{{/}}\n")
		return
	}

	ginkgo.It("transfer in a single node (raw)", func() {
		nativeBalance, err := instances[0].tcli.Balance(context.TODO(), sender, ids.Empty)
		require.NoError(err)
		require.Equal(nativeBalance, startAmount)

		other, err := ed25519.GeneratePrivateKey()
		require.NoError(err)
		aother := auth.NewED25519Address(other.PublicKey())

		ginkgo.By("issue Transfer to the first node", func() {
			// Generate transaction
			parser, err := instances[0].tcli.Parser(context.TODO())
			require.NoError(err)
			submit, tx, maxFee, err := instances[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{&actions.Transfer{
					To:    aother,
					Value: sendAmount,
				}},
				factory,
			)
			require.NoError(err)
			hutils.Outf("{{yellow}}generated transaction{{/}}\n")

			// Broadcast and wait for transaction
			require.NoError(submit(context.Background()))
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, fee, err := instances[0].tcli.WaitForTransaction(ctx, tx.ID())
			cancel()
			require.NoError(err)
			require.True(success)
			hutils.Outf("{{yellow}}found transaction{{/}}\n")

			// Check sender balance
			balance, err := instances[0].tcli.Balance(context.Background(), sender, ids.Empty)
			require.NoError(err)
			hutils.Outf(
				"{{yellow}}start=%d fee=%d send=%d balance=%d{{/}}\n",
				startAmount,
				maxFee,
				sendAmount,
				balance,
			)
			require.Equal(balance, startAmount-fee-sendAmount)
			hutils.Outf("{{yellow}}fetched balance{{/}}\n")
		})

		ginkgo.By("check if Transfer has been accepted from all nodes", func() {
			for _, inst := range instances {
				color.Blue("checking %q", inst.uri)

				// Ensure all blocks processed
				for {
					_, h, _, err := inst.cli.Accepted(context.Background())
					require.NoError(err)
					if h > 0 {
						break
					}
					time.Sleep(1 * time.Second)
				}

				// Check balance of recipient
				balance, err := inst.tcli.Balance(context.Background(), codec.MustAddressBech32(consts.HRP, aother), ids.Empty)
				require.NoError(err)
				require.Equal(balance, sendAmount)
			}
		})
	})

	// TODO: add custom asset test
	// TODO: test with only part of sig weight
	// TODO: attempt to mint a warp asset

	switch mode {
	case modeTest:
		hutils.Outf("{{yellow}}skipping bootstrap and state sync tests{{/}}\n")
		return
	}

	// Create blocks before bootstrapping starts
	count := 0
	ginkgo.It("supports issuance of 128 blocks", func() {
		count += generateBlocks(context.Background(), count, 128, instances, true)
	})

	// Ensure bootstrapping works
	var syncClient *rpc.JSONRPCClient
	var tsyncClient *trpc.JSONRPCClient
	ginkgo.It("can bootstrap a new node", func() {
		cluster, err := anrCli.AddNode(
			context.Background(),
			"bootstrap",
			execPath,
			trackSubnetsOpt,
			runner_sdk.WithChainConfigs(map[string]string{
				blockchainID: vmConfig,
			}),
		)
		require.NoError(err)
		awaitHealthy(anrCli)

		nodeURI := cluster.ClusterInfo.NodeInfos["bootstrap"].Uri
		uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainID)
		bid, err := ids.FromString(blockchainID)
		require.NoError(err)
		hutils.Outf("{{blue}}bootstrap node uri: %s{{/}}\n", uri)
		c := rpc.NewJSONRPCClient(uri)
		syncClient = c
		networkID, _, _, err := syncClient.Network(context.TODO())
		require.NoError(err)
		tc := trpc.NewJSONRPCClient(uri, networkID, bid)
		tsyncClient = tc
		instances = append(instances, instance{
			uri:  uri,
			cli:  c,
			tcli: tc,
		})
	})

	ginkgo.It("accepts transaction after it bootstraps", func() {
		acceptTransaction(syncClient, tsyncClient)
	})

	// Create blocks before state sync starts (state sync requires at least 256
	// blocks)
	//
	// We do 1024 so that there are a number of ranges of data to fetch.
	ginkgo.It("supports issuance of at least 1024 more blocks", func() {
		count += generateBlocks(context.Background(), count, 1024, instances, true)
		// TODO: verify all roots are equal
	})

	ginkgo.It("can state sync a new node when no new blocks are being produced", func() {
		cluster, err := anrCli.AddNode(
			context.Background(),
			"sync",
			execPath,
			trackSubnetsOpt,
			runner_sdk.WithChainConfigs(map[string]string{
				blockchainID: vmConfig,
			}),
		)
		require.NoError(err)

		awaitHealthy(anrCli)

		nodeURI := cluster.ClusterInfo.NodeInfos["sync"].Uri
		uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainID)
		bid, err := ids.FromString(blockchainID)
		require.NoError(err)
		hutils.Outf("{{blue}}sync node uri: %s{{/}}\n", uri)
		syncClient = rpc.NewJSONRPCClient(uri)
		networkID, _, _, err := syncClient.Network(context.TODO())
		require.NoError(err)
		tsyncClient = trpc.NewJSONRPCClient(uri, networkID, bid)
	})

	ginkgo.It("accepts transaction after state sync", func() {
		acceptTransaction(syncClient, tsyncClient)
	})

	ginkgo.It("can pause a node", func() {
		// shuts down the node and keeps all db/conf data for a proper restart
		_, err := anrCli.PauseNode(
			context.Background(),
			"sync",
		)
		require.NoError(err)

		awaitHealthy(anrCli)

		ok, err := syncClient.Ping(context.Background())
		require.Error(err)
		require.False(ok)
	})

	ginkgo.It("supports issuance of 256 more blocks", func() {
		count += generateBlocks(context.Background(), count, 256, instances, true)
		// TODO: verify all roots are equal
	})

	ginkgo.It("can re-sync the restarted node", func() {
		_, err := anrCli.ResumeNode(
			context.Background(),
			"sync",
		)
		require.NoError(err)

		awaitHealthy(anrCli)
	})

	ginkgo.It("accepts transaction after restarted node state sync", func() {
		acceptTransaction(syncClient, tsyncClient)
	})

	ginkgo.It("state sync while broadcasting transactions", func() {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			// Recover failure if exits
			defer ginkgo.GinkgoRecover()

			count += generateBlocks(ctx, count, 0, instances, false)
		}()

		// Give time for transactions to start processing
		time.Sleep(5 * time.Second)

		// Start syncing node
		cluster, err := anrCli.AddNode(
			context.Background(),
			"sync_concurrent",
			execPath,
			trackSubnetsOpt,
			runner_sdk.WithChainConfigs(map[string]string{
				blockchainID: vmConfig,
			}),
		)
		require.NoError(err)
		awaitHealthy(anrCli)

		nodeURI := cluster.ClusterInfo.NodeInfos["sync_concurrent"].Uri
		uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainID)
		bid, err := ids.FromString(blockchainID)
		require.NoError(err)
		hutils.Outf("{{blue}}sync node uri: %s{{/}}\n", uri)
		syncClient = rpc.NewJSONRPCClient(uri)
		networkID, _, _, err := syncClient.Network(context.TODO())
		require.NoError(err)
		tsyncClient = trpc.NewJSONRPCClient(uri, networkID, bid)
		cancel()
	})

	ginkgo.It("accepts transaction after state sync concurrent", func() {
		acceptTransaction(syncClient, tsyncClient)
	})

	// TODO: restart all nodes (crisis simulation)
})

func awaitHealthy(cli runner_sdk.Client) {
	for {
		time.Sleep(healthPollInterval)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := cli.Health(ctx)
		cancel() // by default, health will wait to return until healthy
		if err == nil {
			return
		}
		hutils.Outf(
			"{{yellow}}waiting for health check to pass:{{/}} %v\n",
			err,
		)
	}
}

// generate blocks until either ctx is cancelled or the specified (!= 0) number of blocks is generated.
// if 0 blocks are specified, will just wait until ctx is cancelled.
func generateBlocks(
	ctx context.Context,
	cumulativeTxs int,
	blocksToGenerate uint64,
	instances []instance,
	failOnError bool,
) int {
	require := require.New(ginkgo.GinkgoT())

	_, lastHeight, _, err := instances[0].cli.Accepted(context.Background())
	require.NoError(err)
	parser, err := instances[0].tcli.Parser(context.Background())
	require.NoError(err)
	var targetHeight uint64
	if blocksToGenerate != 0 {
		targetHeight = lastHeight + blocksToGenerate
	}
	for ctx.Err() == nil {
		// Generate transaction
		other, err := ed25519.GeneratePrivateKey()
		require.NoError(err)
		submit, _, _, err := instances[cumulativeTxs%len(instances)].cli.GenerateTransaction(
			context.Background(),
			parser,
			[]chain.Action{&actions.Transfer{
				To:    auth.NewED25519Address(other.PublicKey()),
				Value: 1,
			}},
			factory,
		)
		if failOnError {
			require.NoError(err)
		} else if err != nil {
			hutils.Outf(
				"{{yellow}}unable to generate transaction:{{/}} %v\n",
				err,
			)
			time.Sleep(5 * time.Second)
			continue
		}

		// Broadcast transactions
		err = submit(context.Background())
		if failOnError {
			require.NoError(err)
		} else if err != nil {
			hutils.Outf(
				"{{yellow}}tx broadcast failed:{{/}} %v\n",
				err,
			)
			time.Sleep(5 * time.Second)
			continue
		}
		cumulativeTxs++
		_, height, _, err := instances[0].cli.Accepted(context.Background())
		if failOnError {
			require.NoError(err)
		} else if err != nil {
			hutils.Outf(
				"{{yellow}}height lookup failed:{{/}} %v\n",
				err,
			)
			time.Sleep(5 * time.Second)
			continue
		}
		if targetHeight != 0 && height > targetHeight {
			break
		} else if height > lastHeight {
			lastHeight = height
			hutils.Outf("{{yellow}}height=%d count=%d{{/}}\n", height, cumulativeTxs)
		}

		// Sleep for a very small amount of time to avoid overloading the
		// network with transactions (can generate very fast)
		time.Sleep(10 * time.Millisecond)
	}
	return cumulativeTxs
}

func acceptTransaction(cli *rpc.JSONRPCClient, tcli *trpc.JSONRPCClient) {
	require := require.New(ginkgo.GinkgoT())

	parser, err := tcli.Parser(context.Background())
	require.NoError(err)
	for {
		// Generate transaction
		other, err := ed25519.GeneratePrivateKey()
		require.NoError(err)
		unitPrices, err := cli.UnitPrices(context.Background(), false)
		require.NoError(err)
		submit, tx, maxFee, err := cli.GenerateTransaction(
			context.Background(),
			parser,
			[]chain.Action{&actions.Transfer{
				To:    auth.NewED25519Address(other.PublicKey()),
				Value: sendAmount,
			}},
			factory,
		)
		require.NoError(err)
		hutils.Outf("{{yellow}}generated transaction{{/}} prices: %+v maxFee: %d\n", unitPrices, maxFee)

		// Broadcast and wait for transaction
		require.NoError(submit(context.Background()))
		hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		success, _, err := tcli.WaitForTransaction(ctx, tx.ID())
		cancel()
		if err != nil {
			hutils.Outf("{{red}}cannot find transaction: %v{{/}}\n", err)
			continue
		}
		require.True(success)
		hutils.Outf("{{yellow}}found transaction{{/}}\n")
		break
	}
}

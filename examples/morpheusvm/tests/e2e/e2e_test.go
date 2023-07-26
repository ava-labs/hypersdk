// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e_test

import (
	"context"
	"flag"
	"fmt"
	"testing"
	"time"

	runner_sdk "github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/auth"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	lrpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/utils"
	"github.com/ava-labs/hypersdk/rpc"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/fatih/color"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

const (
	startAmount = uint64(1000000000000)
	sendAmount  = uint64(5000)

	healthPollInterval = 10 * time.Second
)

func TestE2e(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "morpheusvm e2e test suites")
}

var (
	requestTimeout time.Duration

	networkRunnerLogLevel string
	gRPCEp                string
	gRPCGatewayEp         string

	execPath  string
	pluginDir string

	vmGenesisPath    string
	vmConfigPath     string
	subnetConfigPath string
	outputPath       string

	mode string

	logsDir string

	blockchainID string

	trackSubnetsOpt runner_sdk.OpOption
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
}

const (
	modeTest     = "test"
	modeFullTest = "full-test" // runs state sync
	modeRun      = "run"
)

var anrCli runner_sdk.Client

var _ = ginkgo.BeforeSuite(func() {
	gomega.Expect(mode).Should(gomega.Or(
		gomega.Equal(modeTest),
		gomega.Equal(modeFullTest),
		gomega.Equal(modeRun),
	))
	logLevel, err := logging.ToLevel(networkRunnerLogLevel)
	gomega.Expect(err).Should(gomega.BeNil())
	logFactory := logging.NewFactory(logging.Config{
		DisplayLevel: logLevel,
		LogLevel:     logLevel,
	})
	log, err := logFactory.Make("main")
	gomega.Expect(err).Should(gomega.BeNil())

	anrCli, err = runner_sdk.New(runner_sdk.Config{
		Endpoint:    gRPCEp,
		DialTimeout: 10 * time.Second,
	}, log)
	gomega.Expect(err).Should(gomega.BeNil())

	// Load default pk
	priv, err = crypto.HexToKey(
		"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
	)
	gomega.Ω(err).Should(gomega.BeNil())
	factory = auth.NewED25519Factory(priv)
	rsender = priv.PublicKey()
	sender = utils.Address(rsender)
	hutils.Outf("\n{{yellow}}$ loaded address:{{/}} %s\n\n", sender)

	hutils.Outf(
		"{{green}}sending 'start' with binary path:{{/}} %q (%q)\n",
		execPath,
		consts.ID,
	)

	// Start cluster
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	resp, err := anrCli.Start(
		ctx,
		execPath,
		runner_sdk.WithPluginDir(pluginDir),
		// We don't disable PUT gossip here because the E2E test adds multiple
		// non-validating nodes (which will fall behind).
		runner_sdk.WithGlobalNodeConfig(`{
				"log-display-level":"info",
				"proposervm-use-current-height":true,
				"throttler-inbound-validator-alloc-size":"10737418240",
				"throttler-inbound-at-large-alloc-size":"10737418240",
				"throttler-inbound-node-max-processing-msgs":"100000",
				"throttler-inbound-bandwidth-refill-rate":"1073741824",
				"throttler-inbound-bandwidth-max-burst-size":"1073741824",
				"throttler-inbound-cpu-validator-alloc":"100000",
				"throttler-inbound-disk-validator-alloc":"10737418240000",
				"throttler-outbound-validator-alloc-size":"10737418240",
				"throttler-outbound-at-large-alloc-size":"10737418240",
				"consensus-on-accept-gossip-validator-size":"10",
				"consensus-on-accept-gossip-peer-size":"10",
				"network-compression-type":"zstd",
				"consensus-app-concurrency":"512"
			}`),
	)
	cancel()
	gomega.Expect(err).Should(gomega.BeNil())
	hutils.Outf(
		"{{green}}successfully started cluster:{{/}} %s {{green}}subnets:{{/}} %+v\n",
		resp.ClusterInfo.RootDataDir,
		resp.GetClusterInfo().GetSubnets(),
	)
	logsDir = resp.GetClusterInfo().GetRootDataDir()

	// Name 5 new validators (which should have BLS key registered)
	subnet := []string{}
	for i := 1; i <= 5; i++ {
		n := fmt.Sprintf("node%d-bls", i)
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
	gomega.Expect(err).Should(gomega.BeNil())

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

	gomega.Expect(blockchainID).Should(gomega.Not(gomega.BeEmpty()))
	gomega.Expect(logsDir).Should(gomega.Not(gomega.BeEmpty()))

	cctx, ccancel := context.WithTimeout(context.Background(), 2*time.Minute)
	status, err := anrCli.Status(cctx)
	ccancel()
	gomega.Expect(err).Should(gomega.BeNil())
	nodeInfos := status.GetClusterInfo().GetNodeInfos()

	instances = []instance{}
	for _, nodeName := range subnet {
		info := nodeInfos[nodeName]
		u := fmt.Sprintf("%s/ext/bc/%s", info.Uri, blockchainID)
		bid, err := ids.FromString(blockchainID)
		gomega.Expect(err).Should(gomega.BeNil())
		nodeID, err := ids.NodeIDFromString(info.GetId())
		gomega.Expect(err).Should(gomega.BeNil())
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
		gomega.Expect(err).Should(gomega.BeNil())

		instances = append(instances, instance{
			nodeID: nodeID,
			uri:    u,
			cli:    cli,
			lcli:   lrpc.NewJSONRPCClient(u, networkID, bid),
		})
	}
})

var (
	priv    crypto.PrivateKey
	factory *auth.ED25519Factory
	rsender crypto.PublicKey
	sender  string

	instances []instance
)

type instance struct {
	nodeID ids.NodeID
	uri    string
	cli    *rpc.JSONRPCClient
	lcli   *lrpc.JSONRPCClient
}

var _ = ginkgo.AfterSuite(func() {
	switch mode {
	case modeTest, modeFullTest:
		hutils.Outf("{{red}}shutting down cluster{{/}}\n")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		_, err := anrCli.Stop(ctx)
		cancel()
		gomega.Expect(err).Should(gomega.BeNil())

	case modeRun:
		hutils.Outf("{{yellow}}skipping cluster shutdown{{/}}\n\n")
		hutils.Outf("{{cyan}}Blockchain:{{/}} %s\n", blockchainID)
		for _, member := range instances {
			hutils.Outf("%s URI: %s\n", member.nodeID, member.uri)
		}
	}
	gomega.Expect(anrCli.Close()).Should(gomega.BeNil())
})

var _ = ginkgo.Describe("[Ping]", func() {
	ginkgo.It("can ping A", func() {
		for _, inst := range instances {
			cli := inst.cli
			ok, err := cli.Ping(context.Background())
			gomega.Ω(ok).Should(gomega.BeTrue())
			gomega.Ω(err).Should(gomega.BeNil())
		}
	})
})

var _ = ginkgo.Describe("[Network]", func() {
	ginkgo.It("can get network", func() {
		for _, inst := range instances {
			cli := inst.cli
			networkID, _, chainID, err := cli.Network(context.Background())
			gomega.Ω(networkID).Should(gomega.Equal(uint32(1337)))
			gomega.Ω(chainID).ShouldNot(gomega.Equal(ids.Empty))
			gomega.Ω(err).Should(gomega.BeNil())
		}
	})
})

var _ = ginkgo.Describe("[Test]", func() {
	switch mode {
	case modeRun:
		hutils.Outf("{{yellow}}skipping tests{{/}}\n")
		return
	}

	ginkgo.It("get currently accepted block ID", func() {
		for _, inst := range instances {
			cli := inst.cli
			_, h, _, err := cli.Accepted(context.Background())
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(h).Should(gomega.Equal(uint64(0)))
		}
	})

	ginkgo.It("transfer in a single node (raw)", func() {
		nativeBalance, err := instances[0].lcli.Balance(context.TODO(), sender)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(nativeBalance).Should(gomega.Equal(uint64(1000000000000)))

		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		aother := utils.Address(other.PublicKey())

		ginkgo.By("issue Transfer to the first node", func() {
			// Generate transaction
			parser, err := instances[0].lcli.Parser(context.TODO())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, tx, fee, err := instances[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				nil,
				&actions.Transfer{
					To:    other.PublicKey(),
					Value: sendAmount,
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}generated transaction{{/}}\n")

			// Broadcast and wait for transaction
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, err := instances[0].lcli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found transaction{{/}}\n")

			// Check sender balance
			balance, err := instances[0].lcli.Balance(context.Background(), sender)
			gomega.Ω(err).Should(gomega.BeNil())
			hutils.Outf(
				"{{yellow}}start=%d fee=%d send=%d balance=%d{{/}}\n",
				startAmount,
				fee,
				sendAmount,
				balance,
			)
			gomega.Ω(balance).Should(gomega.Equal(startAmount - fee - sendAmount))
			hutils.Outf("{{yellow}}fetched balance{{/}}\n")
		})

		ginkgo.By("check if Transfer has been accepted from all nodes", func() {
			for _, inst := range instances {
				color.Blue("checking %q", inst.uri)

				// Ensure all blocks processed
				for {
					_, h, _, err := inst.cli.Accepted(context.Background())
					gomega.Ω(err).Should(gomega.BeNil())
					if h > 0 {
						break
					}
					time.Sleep(1 * time.Second)
				}

				// Check balance of recipient
				balance, err := inst.lcli.Balance(context.Background(), aother)
				gomega.Ω(err).Should(gomega.BeNil())
				gomega.Ω(balance).Should(gomega.Equal(sendAmount))
			}
		})
	})

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
	var lsyncClient *lrpc.JSONRPCClient
	ginkgo.It("can bootstrap a new node", func() {
		cluster, err := anrCli.AddNode(
			context.Background(),
			"bootstrap",
			execPath,
			trackSubnetsOpt,
		)
		gomega.Expect(err).To(gomega.BeNil())
		awaitHealthy(anrCli, []instance{instances[0]})

		nodeURI := cluster.ClusterInfo.NodeInfos["bootstrap"].Uri
		uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainID)
		bid, err := ids.FromString(blockchainID)
		gomega.Expect(err).To(gomega.BeNil())
		hutils.Outf("{{blue}}bootstrap node uri: %s{{/}}\n", uri)
		c := rpc.NewJSONRPCClient(uri)
		syncClient = c
		networkID, _, _, err := syncClient.Network(context.TODO())
		gomega.Expect(err).Should(gomega.BeNil())
		tc := lrpc.NewJSONRPCClient(uri, networkID, bid)
		lsyncClient = tc
		instances = append(instances, instance{
			uri:  uri,
			cli:  c,
			lcli: tc,
		})
	})

	ginkgo.It("accepts transaction after it bootstraps", func() {
		acceptTransaction(syncClient, lsyncClient)
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
		)
		gomega.Expect(err).To(gomega.BeNil())

		awaitHealthy(anrCli, []instance{instances[0]})

		nodeURI := cluster.ClusterInfo.NodeInfos["sync"].Uri
		uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainID)
		bid, err := ids.FromString(blockchainID)
		gomega.Expect(err).To(gomega.BeNil())
		hutils.Outf("{{blue}}sync node uri: %s{{/}}\n", uri)
		syncClient = rpc.NewJSONRPCClient(uri)
		networkID, _, _, err := syncClient.Network(context.TODO())
		gomega.Expect(err).To(gomega.BeNil())
		lsyncClient = lrpc.NewJSONRPCClient(uri, networkID, bid)
	})

	ginkgo.It("accepts transaction after state sync", func() {
		acceptTransaction(syncClient, lsyncClient)
	})

	ginkgo.It("can pause a node", func() {
		// shuts down the node and keeps all db/conf data for a proper restart
		_, err := anrCli.PauseNode(
			context.Background(),
			"sync",
		)
		gomega.Expect(err).To(gomega.BeNil())

		awaitHealthy(anrCli, []instance{instances[0]})

		ok, err := syncClient.Ping(context.Background())
		gomega.Ω(ok).Should(gomega.BeFalse())
		gomega.Ω(err).Should(gomega.HaveOccurred())
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
		gomega.Expect(err).To(gomega.BeNil())

		awaitHealthy(anrCli, []instance{instances[0]})
	})

	ginkgo.It("accepts transaction after restarted node state sync", func() {
		acceptTransaction(syncClient, lsyncClient)
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
		)
		gomega.Expect(err).To(gomega.BeNil())
		awaitHealthy(anrCli, []instance{instances[0]})

		nodeURI := cluster.ClusterInfo.NodeInfos["sync_concurrent"].Uri
		uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainID)
		bid, err := ids.FromString(blockchainID)
		gomega.Expect(err).To(gomega.BeNil())
		hutils.Outf("{{blue}}sync node uri: %s{{/}}\n", uri)
		syncClient = rpc.NewJSONRPCClient(uri)
		networkID, _, _, err := syncClient.Network(context.TODO())
		gomega.Expect(err).To(gomega.BeNil())
		lsyncClient = lrpc.NewJSONRPCClient(uri, networkID, bid)
		cancel()
	})

	ginkgo.It("accepts transaction after state sync concurrent", func() {
		acceptTransaction(syncClient, lsyncClient)
	})

	// TODO: restart all nodes (crisis simulation)
})

func awaitHealthy(cli runner_sdk.Client, instances []instance) {
	for {
		time.Sleep(healthPollInterval)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := cli.Health(ctx)
		cancel() // by default, health will wait to return until healthy
		if err == nil {
			return
		}
		hutils.Outf(
			"{{yellow}}waiting for health check to pass...broadcasting tx while waiting:{{/}} %v\n",
			err,
		)

		// Add more txs via other nodes until healthy (should eventually happen after
		// [ValidityWindow] processed)
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		for _, instance := range instances {
			parser, err := instance.lcli.Parser(context.Background())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, _, _, err := instance.cli.GenerateTransaction(
				context.Background(),
				parser,
				nil,
				&actions.Transfer{
					To:    other.PublicKey(),
					Value: 1,
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			// Broadcast and wait for transaction
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		}
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
	_, lastHeight, _, err := instances[0].cli.Accepted(context.Background())
	gomega.Ω(err).Should(gomega.BeNil())
	parser, err := instances[0].lcli.Parser(context.Background())
	gomega.Ω(err).Should(gomega.BeNil())
	var targetHeight uint64
	if blocksToGenerate != 0 {
		targetHeight = lastHeight + blocksToGenerate
	}
	for ctx.Err() == nil {
		// Generate transaction
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[cumulativeTxs%len(instances)].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.Transfer{
				To:    other.PublicKey(),
				Value: 1,
			},
			factory,
		)
		if failOnError {
			gomega.Ω(err).Should(gomega.BeNil())
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
			gomega.Ω(err).Should(gomega.BeNil())
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
			gomega.Ω(err).Should(gomega.BeNil())
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

func acceptTransaction(cli *rpc.JSONRPCClient, lcli *lrpc.JSONRPCClient) {
	parser, err := lcli.Parser(context.Background())
	gomega.Ω(err).Should(gomega.BeNil())
	for {
		// Generate transaction
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		submit, tx, _, err := cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.Transfer{
				To:    other.PublicKey(),
				Value: sendAmount,
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		hutils.Outf("{{yellow}}generated transaction{{/}}\n")

		// Broadcast and wait for transaction
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		success, err := lcli.WaitForTransaction(ctx, tx.ID())
		cancel()
		if err != nil {
			hutils.Outf("{{red}}cannot find transaction: %v{{/}}\n", err)
			continue
		}
		gomega.Ω(success).Should(gomega.BeTrue())
		hutils.Outf("{{yellow}}found transaction{{/}}\n")
		break
	}
}

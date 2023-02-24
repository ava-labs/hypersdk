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

	runner_sdk "github.com/ava-labs/avalanche-network-runner/client"
	"github.com/ava-labs/avalanche-network-runner/rpcpb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/fatih/color"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"sigs.k8s.io/yaml"
)

const (
	startAmount = uint64(1000000000000)
	sendAmount  = uint64(5000)

	healthPollInterval = 10 * time.Second
)

func TestE2e(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "tokenvm e2e test suites")
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

	blockchainID string
	logsDir      string
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

var (
	cli           runner_sdk.Client
	tokenvmRPCEps []string
)

var _ = ginkgo.BeforeSuite(func() {
	gomega.Expect(mode).
		Should(gomega.Or(gomega.Equal("test"), gomega.Equal("full-test"), gomega.Equal("run")))
	logLevel, err := logging.ToLevel(networkRunnerLogLevel)
	gomega.Expect(err).Should(gomega.BeNil())
	logFactory := logging.NewFactory(logging.Config{
		DisplayLevel: logLevel,
		LogLevel:     logLevel,
	})
	log, err := logFactory.Make("main")
	gomega.Expect(err).Should(gomega.BeNil())

	cli, err = runner_sdk.New(runner_sdk.Config{
		Endpoint:    gRPCEp,
		DialTimeout: 10 * time.Second,
	}, log)
	gomega.Expect(err).Should(gomega.BeNil())

	ginkgo.By("calling start API via network runner", func() {
		hutils.Outf(
			"{{green}}sending 'start' with binary path:{{/}} %q (%q)\n",
			execPath,
			consts.ID,
		)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		resp, err := cli.Start(
			ctx,
			execPath,
			runner_sdk.WithPluginDir(pluginDir),
			runner_sdk.WithBlockchainSpecs(
				[]*rpcpb.BlockchainSpec{
					{
						VmName:       consts.Name,
						Genesis:      vmGenesisPath,
						ChainConfig:  vmConfigPath,
						SubnetConfig: subnetConfigPath,
					},
				},
			),
			runner_sdk.WithGlobalNodeConfig(`{
				"log-display-level":"info",
				"proposervm-use-current-height":true,
				"throttler-inbound-validator-alloc-size":"107374182",
				"throttler-inbound-node-max-processing-msgs":"100000",
				"throttler-inbound-bandwidth-refill-rate":"1073741824",
				"throttler-inbound-bandwidth-max-burst-size":"1073741824",
				"throttler-inbound-cpu-validator-alloc":"100000",
				"throttler-inbound-disk-validator-alloc":"10737418240000",
				"throttler-outbound-validator-alloc-size":"107374182"
			}`),
		)
		cancel()
		gomega.Expect(err).Should(gomega.BeNil())
		hutils.Outf("{{green}}successfully started:{{/}} %s\n", resp.ClusterInfo.RootDataDir)
	})

	// TODO: network runner health should imply custom VM healthiness
	// or provide a separate API for custom VM healthiness
	// "start" is async, so wait some time for cluster health
	hutils.Outf("\n{{magenta}}waiting for all vms to report healthy...{{/}}: %s\n", consts.ID)
	for {
		v, err := cli.Health(context.Background())
		hutils.Outf("\n{{yellow}}health result{{/}}: %+v %s\n", v, err)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		// TODO: clean this up
		gomega.Expect(err).Should(gomega.BeNil())
		break
	}

	tokenvmRPCEps = make([]string, 0)

	// wait up to 5-minute for custom VM installation
	hutils.Outf("\n{{magenta}}waiting for all custom VMs to report healthy...{{/}}\n")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	for ctx.Err() == nil && len(blockchainID) == 0 {
		select {
		case <-ctx.Done():
			gomega.Expect(ctx.Err()).Should(gomega.BeNil())
		case <-time.After(5 * time.Second):
		}

		hutils.Outf("{{magenta}}checking custom VM status{{/}}\n")
		cctx, ccancel := context.WithTimeout(context.Background(), 2*time.Minute)
		resp, err := cli.Status(cctx)
		ccancel()
		gomega.Expect(err).Should(gomega.BeNil())

		// all logs are stored under root data dir
		logsDir = resp.GetClusterInfo().GetRootDataDir()

		for _, v := range resp.ClusterInfo.CustomChains {
			if v.VmId == consts.ID.String() {
				blockchainID = v.ChainId
				hutils.Outf("{{blue}}tokenvm is ready:{{/}} %+v\n", v)
				break
			}
		}
	}
	gomega.Expect(ctx.Err()).Should(gomega.BeNil())
	cancel()

	gomega.Expect(blockchainID).Should(gomega.Not(gomega.BeEmpty()))
	gomega.Expect(logsDir).Should(gomega.Not(gomega.BeEmpty()))

	cctx, ccancel := context.WithTimeout(context.Background(), 2*time.Minute)
	uris, err := cli.URIs(cctx)
	ccancel()
	gomega.Expect(err).Should(gomega.BeNil())
	hutils.Outf("{{blue}}avalanche HTTP RPCs URIs:{{/}} %q\n", uris)

	for _, u := range uris {
		rpcEP := fmt.Sprintf("%s/ext/bc/%s/rpc", u, blockchainID)
		tokenvmRPCEps = append(tokenvmRPCEps, rpcEP)
		hutils.Outf("{{blue}}avalanche tokenvm RPC:{{/}} %q\n", rpcEP)
	}

	pid := os.Getpid()
	hutils.Outf("{{blue}}{{bold}}writing output %q with PID %d{{/}}\n", outputPath, pid)
	ci := clusterInfo{
		URIs:     uris,
		Endpoint: fmt.Sprintf("/ext/bc/%s", blockchainID),
		PID:      pid,
		LogsDir:  logsDir,
	}
	gomega.Expect(ci.Save(outputPath)).Should(gomega.BeNil())

	b, err := os.ReadFile(outputPath)
	gomega.Expect(err).Should(gomega.BeNil())
	hutils.Outf("\n{{blue}}$ cat %s:{{/}}\n%s\n", outputPath, string(b))

	priv, err = crypto.HexToKey(
		"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
	)
	gomega.Ω(err).Should(gomega.BeNil())
	factory = auth.NewED25519Factory(priv)
	rsender = priv.PublicKey()
	sender = utils.Address(rsender)
	hutils.Outf("\n{{yellow}}$ loaded address %s:{{/}}\n", sender)

	instances = make([]instance, len(uris))
	for i := range uris {
		u := uris[i] + fmt.Sprintf("/ext/bc/%s", blockchainID)
		instances[i] = instance{
			uri: u,
			cli: client.New(u),
		}
	}
	gen, err = instances[0].cli.Genesis(context.Background())
	gomega.Ω(err).Should(gomega.BeNil())
})

var (
	priv    crypto.PrivateKey
	factory *auth.ED25519Factory
	rsender crypto.PublicKey
	sender  string

	instances []instance

	gen *genesis.Genesis
)

type instance struct {
	uri string
	cli *client.Client
}

var _ = ginkgo.AfterSuite(func() {
	switch mode {
	case modeTest, modeFullTest:
		hutils.Outf("{{red}}shutting down cluster{{/}}\n")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		_, err := cli.Stop(ctx)
		cancel()
		gomega.Expect(err).Should(gomega.BeNil())

	case modeRun:
		hutils.Outf("{{yellow}}skipping shutting down cluster{{/}}\n")
	}

	hutils.Outf("{{red}}shutting down client{{/}}\n")
	gomega.Expect(cli.Close()).Should(gomega.BeNil())
})

var _ = ginkgo.Describe("[Ping]", func() {
	ginkgo.It("can ping", func() {
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

var _ = ginkgo.Describe("[Transfer]", func() {
	switch mode {
	case modeRun:
		hutils.Outf("{{yellow}}skipping Transfer tests{{/}}\n")
		return
	}

	ginkgo.It("get currently accepted block ID", func() {
		for _, inst := range instances {
			cli := inst.cli
			_, h, err := cli.Accepted(context.Background())
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(h).Should(gomega.Equal(uint64(0)))
		}
	})

	ginkgo.It("transfer in a single node (raw)", func() {
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		aother := utils.Address(other.PublicKey())

		ginkgo.By("issue Transfer to the first node", func() {
			// Generate transaction
			submit, tx, fee, err := instances[0].cli.GenerateTransaction(
				context.Background(),
				&actions.Transfer{
					To:    other.PublicKey(),
					Value: sendAmount, // must be more than state lockup
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}generated transaction{{/}}\n")

			// Broadcast and wait for transaction
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, err := instances[0].cli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found transaction{{/}}\n")

			// Check sender balance
			balance, err := instances[0].cli.Balance(context.Background(), sender, ids.Empty)
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
					_, h, err := inst.cli.Accepted(context.Background())
					gomega.Ω(err).Should(gomega.BeNil())
					if h > 0 {
						break
					}
					time.Sleep(1 * time.Second)
				}

				// Check balance of recipient
				balance, err := inst.cli.Balance(context.Background(), aother, ids.Empty)
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
	lastHeight := uint64(0)
	ginkgo.It("supports issuance of 128 blocks", func() {
		for {
			// Generate transaction
			other, err := crypto.GeneratePrivateKey()
			gomega.Ω(err).Should(gomega.BeNil())
			submit, _, _, err := instances[count%len(instances)].cli.GenerateTransaction(
				context.Background(),
				&actions.Transfer{
					To:    other.PublicKey(),
					Value: 1,
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())

			// Broadcast and wait for transaction
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			count++
			_, height, err := instances[0].cli.Accepted(context.Background())
			gomega.Ω(err).Should(gomega.BeNil())
			if height > 128 {
				break
			} else {
				if height > lastHeight {
					lastHeight = height
					hutils.Outf("{{yellow}}height=%d count=%d{{/}}\n", height, count)
				}
				time.Sleep(5 * time.Millisecond)
			}
		}
	})

	// Ensure bootstrapping works
	var syncClient *client.Client
	ginkgo.It("can bootstrap a new node", func() {
		cluster, err := cli.AddNode(
			context.Background(),
			"bootstrap",
			execPath,
			runner_sdk.WithPluginDir(pluginDir),
			runner_sdk.WithBlockchainSpecs(
				[]*rpcpb.BlockchainSpec{
					{
						VmName:       consts.Name,
						Genesis:      vmGenesisPath,
						ChainConfig:  vmConfigPath,
						SubnetConfig: subnetConfigPath,
					},
				},
			),
			runner_sdk.WithGlobalNodeConfig(`{
				"log-display-level":"info",
				"proposervm-use-current-height":true,
				"throttler-inbound-validator-alloc-size":"107374182",
				"throttler-inbound-node-max-processing-msgs":"100000",
				"throttler-inbound-bandwidth-refill-rate":"1073741824",
				"throttler-inbound-bandwidth-max-burst-size":"1073741824",
				"throttler-inbound-cpu-validator-alloc":"100000",
				"throttler-inbound-disk-validator-alloc":"10737418240000",
				"throttler-outbound-validator-alloc-size":"107374182"
			}`),
		)
		gomega.Expect(err).To(gomega.BeNil())
		awaitHealthy(cli)

		nodeURI := cluster.ClusterInfo.NodeInfos["bootstrap"].Uri
		uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainID)
		hutils.Outf("{{blue}}bootstrap node uri: %s{{/}}\n", uri)
		c := client.New(uri)
		syncClient = c
		instances = append(instances, instance{
			uri: uri,
			cli: c,
		})
	})

	ginkgo.It("accepts transaction after it bootstraps", func() {
		for {
			// Generate transaction
			other, err := crypto.GeneratePrivateKey()
			gomega.Ω(err).Should(gomega.BeNil())
			submit, tx, _, err := syncClient.GenerateTransaction(
				context.Background(),
				&actions.Transfer{
					To:    other.PublicKey(),
					Value: sendAmount, // must be more than state lockup
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}generated transaction{{/}}\n")

			// Broadcast and wait for transaction
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, err := syncClient.WaitForTransaction(ctx, tx.ID())
			cancel()
			if err != nil {
				hutils.Outf("{{red}}cannot find transaction: %v{{/}}\n", err)
				continue
			}
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found transaction{{/}}\n")
			break
		}
	})

	// Create blocks before state sync starts (state sync requires at least 256
	// blocks)
	//
	// We do 1024 so that there are a number of ranges of data to fetch.
	ginkgo.It("supports issuance of at least 1024 more blocks", func() {
		for {
			// Generate transaction
			other, err := crypto.GeneratePrivateKey()
			gomega.Ω(err).Should(gomega.BeNil())
			submit, _, _, err := instances[count%len(instances)].cli.GenerateTransaction(
				context.Background(),
				&actions.Transfer{
					To:    other.PublicKey(),
					Value: 1,
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())

			// Broadcast and wait for transaction
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			count++
			_, height, err := instances[0].cli.Accepted(context.Background())
			gomega.Ω(err).Should(gomega.BeNil())
			if height > 128+1024 {
				break
			} else {
				if height > lastHeight {
					lastHeight = height
					hutils.Outf("{{yellow}}height=%d count=%d{{/}}\n", height, count)
				}
				time.Sleep(5 * time.Millisecond)
			}
		}

		// TODO: verify all roots are equal
	})

	ginkgo.It("can state sync a new node when no new blocks are being produced", func() {
		cluster, err := cli.AddNode(
			context.Background(),
			"sync",
			execPath,
			runner_sdk.WithPluginDir(pluginDir),
			runner_sdk.WithBlockchainSpecs(
				[]*rpcpb.BlockchainSpec{
					{
						VmName:       consts.Name,
						Genesis:      vmGenesisPath,
						ChainConfig:  vmConfigPath,
						SubnetConfig: subnetConfigPath,
					},
				},
			),
			runner_sdk.WithGlobalNodeConfig(`{
				"log-display-level":"info",
				"proposervm-use-current-height":true,
				"throttler-inbound-validator-alloc-size":"107374182",
				"throttler-inbound-node-max-processing-msgs":"100000",
				"throttler-inbound-bandwidth-refill-rate":"1073741824",
				"throttler-inbound-bandwidth-max-burst-size":"1073741824",
				"throttler-inbound-cpu-validator-alloc":"100000",
				"throttler-inbound-disk-validator-alloc":"10737418240000",
				"throttler-outbound-validator-alloc-size":"107374182"
			}`),
		)
		gomega.Expect(err).To(gomega.BeNil())

		// Wait some time before broadcasting new txs so that state sync can finsih
		// without updating roots
		//
		// TODO: find a way to communicate this explicitly without sleeping
		time.Sleep(30 * time.Second) // assume state sync takes less than this to complete
		awaitHealthy(cli)

		nodeURI := cluster.ClusterInfo.NodeInfos["sync"].Uri
		uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainID)
		hutils.Outf("{{blue}}sync node uri: %s{{/}}\n", uri)
		syncClient = client.New(uri)
	})

	ginkgo.It("accepts transaction after state sync", func() {
		for {
			// Generate transaction
			other, err := crypto.GeneratePrivateKey()
			gomega.Ω(err).Should(gomega.BeNil())
			submit, tx, _, err := syncClient.GenerateTransaction(
				context.Background(),
				&actions.Transfer{
					To:    other.PublicKey(),
					Value: sendAmount, // must be more than state lockup
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}generated transaction{{/}}\n")

			// Broadcast and wait for transaction
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, err := syncClient.WaitForTransaction(ctx, tx.ID())
			cancel()
			if err != nil {
				hutils.Outf("{{red}}cannot find transaction: %v{{/}}\n", err)
				continue
			}
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found transaction{{/}}\n")
			break
		}
	})

	ginkgo.It("state sync while broadcasting transactions", func() {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			for ctx.Err() == nil {
				other, err := crypto.GeneratePrivateKey()
				gomega.Ω(err).Should(gomega.BeNil())
				submit, _, _, err := instances[count%len(instances)].cli.GenerateTransaction(
					context.Background(),
					&actions.Transfer{
						To:    other.PublicKey(),
						Value: 1,
					},
					factory,
				)
				gomega.Ω(err).Should(gomega.BeNil())

				// Broadcast and wait for transaction
				gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
				count++
				_, height, err := instances[0].cli.Accepted(context.Background())
				gomega.Ω(err).Should(gomega.BeNil())
				if height > lastHeight {
					lastHeight = height
					hutils.Outf("{{yellow}}height=%d count=%d{{/}}\n", height, count)
				}
				time.Sleep(5 * time.Millisecond)
			}
		}()

		// Give time for transactions to start processing
		time.Sleep(5 * time.Second)

		// Start syncing node
		cluster, err := cli.AddNode(
			context.Background(),
			"sync_concurrent",
			execPath,
			runner_sdk.WithPluginDir(pluginDir),
			runner_sdk.WithBlockchainSpecs(
				[]*rpcpb.BlockchainSpec{
					{
						VmName:       consts.Name,
						Genesis:      vmGenesisPath,
						ChainConfig:  vmConfigPath,
						SubnetConfig: subnetConfigPath,
					},
				},
			),
			runner_sdk.WithGlobalNodeConfig(`{
				"log-display-level":"info",
				"proposervm-use-current-height":true,
				"throttler-inbound-validator-alloc-size":"107374182",
				"throttler-inbound-node-max-processing-msgs":"100000",
				"throttler-inbound-bandwidth-refill-rate":"1073741824",
				"throttler-inbound-bandwidth-max-burst-size":"1073741824",
				"throttler-inbound-cpu-validator-alloc":"100000",
				"throttler-inbound-disk-validator-alloc":"10737418240000",
				"throttler-outbound-validator-alloc-size":"107374182"
			}`),
		)
		gomega.Expect(err).To(gomega.BeNil())
		awaitHealthy(cli)

		nodeURI := cluster.ClusterInfo.NodeInfos["sync_concurrent"].Uri
		uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainID)
		hutils.Outf("{{blue}}sync node uri: %s{{/}}\n", uri)
		syncClient = client.New(uri)
		cancel()
	})

	ginkgo.It("accepts transaction after state sync concurrent", func() {
		for {
			// Generate transaction
			other, err := crypto.GeneratePrivateKey()
			gomega.Ω(err).Should(gomega.BeNil())
			submit, tx, _, err := syncClient.GenerateTransaction(
				context.Background(),
				&actions.Transfer{
					To:    other.PublicKey(),
					Value: sendAmount, // must be more than state lockup
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())

			// Broadcast and wait for transaction
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, err := syncClient.WaitForTransaction(ctx, tx.ID())
			cancel()
			if err != nil {
				hutils.Outf("{{red}}cannot find transaction: %v{{/}}\n", err)
				continue
			}
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found transaction{{/}}\n")
			break
		}
	})

	// TODO: restart synced node + process blocks + re-sync
	// TODO: restart all nodes (crisis simulation)
})

// clusterInfo represents the local cluster information.
type clusterInfo struct {
	URIs     []string `json:"uris"`
	Endpoint string   `json:"endpoint"`
	PID      int      `json:"pid"`
	LogsDir  string   `json:"logsDir"`
}

const fsModeWrite = 0o600

func (ci clusterInfo) Save(p string) error {
	ob, err := yaml.Marshal(ci)
	if err != nil {
		return err
	}
	return os.WriteFile(p, ob, fsModeWrite)
}

func awaitHealthy(cli runner_sdk.Client) {
	for {
		time.Sleep(healthPollInterval)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		_, err := cli.Health(ctx)
		cancel() // by default, health will wait to return until healthy
		if err != nil {
			hutils.Outf(
				"{{yellow}}waiting for health check to pass...broadcasting tx while waiting:{{/}} %v\n",
				err,
			)

			// Add more txs via other nodes until healthy (should eventually happen after
			// [ValidityWindow] processed)
			other, err := crypto.GeneratePrivateKey()
			gomega.Ω(err).Should(gomega.BeNil())
			submit, _, _, err := instances[0].cli.GenerateTransaction(
				context.Background(),
				&actions.Transfer{
					To:    other.PublicKey(),
					Value: 1,
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())

			// Broadcast and wait for transaction
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			continue
		}
		return
	}
}

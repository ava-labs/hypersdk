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
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
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

	logsDir string

	blockchainIDA string
	blockchainIDB string
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

	anrCli, err = runner_sdk.New(runner_sdk.Config{
		Endpoint:    gRPCEp,
		DialTimeout: 10 * time.Second,
	}, log)
	gomega.Expect(err).Should(gomega.BeNil())

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
	hutils.Outf(
		"{{green}}successfully started cluster:{{/}} %s {{green}}subnets:{{/}} %+v\n",
		resp.ClusterInfo.RootDataDir,
		resp.GetClusterInfo().GetSubnets(),
	)
	logsDir = resp.GetClusterInfo().GetRootDataDir()

	// Name 10 new validators (which should have BLS key registered)
	subnetA := []string{}
	subnetB := []string{}
	for i := 1; i <= 10; i++ {
		n := fmt.Sprintf("node%d-bls", i)
		if i <= 5 {
			subnetA = append(subnetA, n)
		} else {
			subnetB = append(subnetB, n)
		}
	}

	// Create 2 subnets
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Minute)
	sresp, err := anrCli.CreateSpecificBlockchains(
		ctx,
		execPath,
		[]*rpcpb.BlockchainSpec{
			{
				VmName:       consts.Name,
				Genesis:      vmGenesisPath,
				ChainConfig:  vmConfigPath,
				SubnetConfig: subnetConfigPath,
				Participants: subnetA,
			},
			{
				VmName:       consts.Name,
				Genesis:      vmGenesisPath,
				ChainConfig:  vmConfigPath,
				SubnetConfig: subnetConfigPath,
				Participants: subnetB,
			},
		},
	)
	cancel()
	gomega.Expect(err).Should(gomega.BeNil())

	blockchainIDA = sresp.Chains[0]
	hutils.Outf(
		"{{green}}successfully added chain:{{/}} %s {{green}}subnet:{{/}} %s {{green}}participants:{{/}} %+v\n",
		blockchainIDA,
		sresp.ClusterInfo.CustomChains[blockchainIDA].SubnetId,
		subnetA,
	)
	blockchainIDB = sresp.Chains[1]
	hutils.Outf(
		"{{green}}successfully added chain:{{/}} %s {{green}}subnet:{{/}} %s {{green}}participants:{{/}} %+v\n",
		blockchainIDB,
		sresp.ClusterInfo.CustomChains[blockchainIDB].SubnetId,
		subnetB,
	)

	// TODO: network runner health should imply custom VM healthiness
	// or provide a separate API for custom VM healthiness
	// "start" is async, so wait some time for cluster health
	hutils.Outf("\n{{magenta}}waiting for all vms to report healthy...{{/}}: %s\n", consts.ID)
	for {
		v, err := anrCli.Health(context.Background())
		hutils.Outf("\n{{yellow}}health result{{/}}: %+v %s\n", v, err)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}
		// TODO: clean this up
		gomega.Expect(err).Should(gomega.BeNil())
		break
	}

	gomega.Expect(blockchainIDA).Should(gomega.Not(gomega.BeEmpty()))
	gomega.Expect(blockchainIDB).Should(gomega.Not(gomega.BeEmpty()))
	gomega.Expect(logsDir).Should(gomega.Not(gomega.BeEmpty()))

	cctx, ccancel := context.WithTimeout(context.Background(), 2*time.Minute)
	status, err := anrCli.Status(cctx)
	ccancel()
	gomega.Expect(err).Should(gomega.BeNil())
	nodeInfos := status.GetClusterInfo().GetNodeInfos()

	instancesA = []instance{}
	for _, nodeName := range subnetA {
		u := fmt.Sprintf("%s/ext/bc/%s", nodeInfos[nodeName].Uri, blockchainIDA)
		instancesA = append(instancesA, instance{
			uri: u,
			cli: client.New(u),
		})
	}
	instancesB = []instance{}
	for _, nodeName := range subnetB {
		u := fmt.Sprintf("%s/ext/bc/%s", nodeInfos[nodeName].Uri, blockchainIDB)
		instancesB = append(instancesB, instance{
			uri: u,
			cli: client.New(u),
		})
	}

	priv, err = crypto.HexToKey(
		"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
	)
	gomega.Ω(err).Should(gomega.BeNil())
	factory = auth.NewED25519Factory(priv)
	rsender = priv.PublicKey()
	sender = utils.Address(rsender)
	hutils.Outf("\n{{yellow}}$ loaded address:{{/}} %s\n\n", sender)

	gen, err = instancesA[0].cli.Genesis(context.Background())
	gomega.Ω(err).Should(gomega.BeNil())
})

var (
	priv    crypto.PrivateKey
	factory *auth.ED25519Factory
	rsender crypto.PublicKey
	sender  string

	instancesA []instance
	instancesB []instance

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
		_, err := anrCli.Stop(ctx)
		cancel()
		gomega.Expect(err).Should(gomega.BeNil())

	case modeRun:
		hutils.Outf("{{yellow}}skipping cluster shutdown{{/}}\n\n")
		hutils.Outf("{{cyan}}Blockchain:{{/}} %s\n", blockchainIDA)
		for _, member := range instancesA {
			hutils.Outf("URI: %s\n", member.uri)
		}
		hutils.Outf("\n{{cyan}}Blockchain:{{/}} %s\n", blockchainIDB)
		for _, member := range instancesB {
			hutils.Outf("URI: %s\n", member.uri)
		}
	}
	gomega.Expect(anrCli.Close()).Should(gomega.BeNil())
})

var _ = ginkgo.Describe("[Ping]", func() {
	ginkgo.It("can ping A", func() {
		for _, inst := range instancesA {
			cli := inst.cli
			ok, err := cli.Ping(context.Background())
			gomega.Ω(ok).Should(gomega.BeTrue())
			gomega.Ω(err).Should(gomega.BeNil())
		}
	})

	ginkgo.It("can ping B", func() {
		for _, inst := range instancesB {
			cli := inst.cli
			ok, err := cli.Ping(context.Background())
			gomega.Ω(ok).Should(gomega.BeTrue())
			gomega.Ω(err).Should(gomega.BeNil())
		}
	})
})

var _ = ginkgo.Describe("[Network]", func() {
	ginkgo.It("can get network A", func() {
		for _, inst := range instancesA {
			cli := inst.cli
			networkID, _, chainID, err := cli.Network(context.Background())
			gomega.Ω(networkID).Should(gomega.Equal(uint32(1337)))
			gomega.Ω(chainID).ShouldNot(gomega.Equal(ids.Empty))
			gomega.Ω(err).Should(gomega.BeNil())
		}
	})

	ginkgo.It("can get network B", func() {
		for _, inst := range instancesB {
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
		hutils.Outf("{{yellow}}skipping Transfer tests{{/}}\n")
		return
	}

	ginkgo.It("get currently accepted block ID", func() {
		for _, inst := range instancesA {
			cli := inst.cli
			_, h, _, err := cli.Accepted(context.Background())
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(h).Should(gomega.Equal(uint64(0)))
		}
	})

	ginkgo.It("transfer in a single node (raw)", func() {
		nativeBalance, err := instancesA[0].cli.Balance(context.TODO(), sender, ids.Empty)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(nativeBalance).Should(gomega.Equal(uint64(1000000000000)))

		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		aother := utils.Address(other.PublicKey())

		ginkgo.By("issue Transfer to the first node", func() {
			// Generate transaction
			submit, tx, fee, err := instancesA[0].cli.GenerateTransaction(
				context.Background(),
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
			success, err := instancesA[0].cli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found transaction{{/}}\n")

			// Check sender balance
			balance, err := instancesA[0].cli.Balance(context.Background(), sender, ids.Empty)
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
			for _, inst := range instancesA {
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
				balance, err := inst.cli.Balance(context.Background(), aother, ids.Empty)
				gomega.Ω(err).Should(gomega.BeNil())
				gomega.Ω(balance).Should(gomega.Equal(sendAmount))
			}
		})
	})

	ginkgo.It("performs a warp transfer of the native asset", func() {
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		aother := utils.Address(other.PublicKey())
		source, err := ids.FromString(blockchainIDA)
		gomega.Ω(err).Should(gomega.BeNil())
		destination, err := ids.FromString(blockchainIDB)
		gomega.Ω(err).Should(gomega.BeNil())
		otherFactory := auth.NewED25519Factory(other)

		var txID ids.ID
		ginkgo.By("submitting an export action on source", func() {
			otherBalance, err := instancesA[0].cli.Balance(context.Background(), aother, ids.Empty)
			gomega.Ω(err).Should(gomega.BeNil())
			senderBalance, err := instancesA[0].cli.Balance(context.Background(), sender, ids.Empty)
			gomega.Ω(err).Should(gomega.BeNil())

			submit, tx, fees, err := instancesA[0].cli.GenerateTransaction(
				context.Background(),
				nil,
				&actions.ExportAsset{
					To:          other.PublicKey(),
					Asset:       ids.Empty,
					Value:       sendAmount,
					Return:      false,
					Destination: destination,
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			txID = tx.ID()
			hutils.Outf("{{yellow}}generated transaction:{{/}} %s\n", txID)

			// Broadcast and wait for transaction
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, err := instancesA[0].cli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp export transaction{{/}}\n")

			// Check loans and balances
			amount, err := instancesA[0].cli.Loan(context.Background(), ids.Empty, destination)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(amount).Should(gomega.Equal(sendAmount))
			aotherBalance, err := instancesA[0].cli.Balance(context.Background(), aother, ids.Empty)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(otherBalance).Should(gomega.Equal(aotherBalance))
			asenderBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(asenderBalance).Should(gomega.Equal(senderBalance - sendAmount - fees))
		})

		ginkgo.By(
			"ensuring snowman++ is activated on destination + fund other account with native",
			func() {
				for i := 0; i < 10; i++ {
					submit, tx, _, err := instancesB[0].cli.GenerateTransaction(
						context.Background(),
						nil,
						&actions.Transfer{
							To:    other.PublicKey(),
							Asset: ids.Empty,
							Value: 1000 + uint64(i), // ensure we don't produce same tx
						},
						factory,
					)
					gomega.Ω(err).Should(gomega.BeNil())
					hutils.Outf("{{yellow}}generated transaction:{{/}} %s\n", txID)

					// Broadcast and wait for transaction
					gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
					hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
					ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
					success, err := instancesB[0].cli.WaitForTransaction(ctx, tx.ID())
					cancel()
					gomega.Ω(err).Should(gomega.BeNil())
					gomega.Ω(success).Should(gomega.BeTrue())
					hutils.Outf("{{yellow}}found transaction on B{{/}}\n")
				}
			},
		)

		ginkgo.By("submitting an import action on destination", func() {
			bIDA, err := ids.FromString(blockchainIDA)
			gomega.Ω(err).Should(gomega.BeNil())
			newAsset := actions.ImportedAssetID(ids.Empty, bIDA)
			nativeOtherBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newOtherBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(newOtherBalance).Should(gomega.Equal(uint64(0)))
			nativeSenderBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newSenderBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				sender,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(newSenderBalance).Should(gomega.Equal(uint64(0)))

			var (
				msg                     *warp.Message
				subnetWeight, sigWeight uint64
			)
			for {
				msg, subnetWeight, sigWeight, err = instancesA[0].cli.GenerateAggregateWarpSignature(
					context.Background(),
					txID,
				)
				if sigWeight == subnetWeight && err == nil {
					break
				}
				if err == nil {
					hutils.Outf(
						"{{yellow}}waiting for signature weight:{{/}} %d {{yellow}}observed:{{/}} %d\n",
						subnetWeight,
						sigWeight,
					)
				} else {
					hutils.Outf("{{red}}found error:{{/}} %v\n", err)
				}
				time.Sleep(1 * time.Second)
			}
			hutils.Outf(
				"{{green}}fetched signature weight:{{/}} %d {{green}}total weight:{{/}} %d\n",
				sigWeight,
				subnetWeight,
			)
			gomega.Ω(subnetWeight).Should(gomega.Equal(sigWeight))

			submit, tx, fees, err := instancesB[0].cli.GenerateTransaction(
				context.Background(),
				msg,
				&actions.ImportAsset{},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			txID = tx.ID()
			hutils.Outf("{{yellow}}generated transaction:{{/}} %s\n", txID)
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, err := instancesB[0].cli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp import transaction{{/}}\n")

			// Check asset info and balance
			aNativeOtherBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(nativeOtherBalance).Should(gomega.Equal(aNativeOtherBalance))
			aNewOtherBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNewOtherBalance).Should(gomega.Equal(sendAmount))
			aNativeSenderBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNativeSenderBalance).Should(gomega.Equal(nativeSenderBalance - fees))
			aNewSenderBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				sender,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNewSenderBalance).Should(gomega.Equal(uint64(0)))
			exists, metadata, supply, owner, warp, err := instancesB[0].cli.Asset(
				context.Background(),
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(exists).Should(gomega.BeTrue())
			gomega.Ω(metadata).Should(gomega.Equal(actions.ImportedAssetMetadata(ids.Empty, bIDA)))
			gomega.Ω(supply).Should(gomega.Equal(sendAmount))
			gomega.Ω(owner).Should(gomega.Equal(utils.Address(crypto.EmptyPublicKey)))
			gomega.Ω(warp).Should(gomega.BeTrue())
		})

		ginkgo.By("submitting an invalid export action to new destination", func() {
			bIDA, err := ids.FromString(blockchainIDA)
			gomega.Ω(err).Should(gomega.BeNil())
			newAsset := actions.ImportedAssetID(ids.Empty, bIDA)
			submit, tx, _, err := instancesB[0].cli.GenerateTransaction(
				context.Background(),
				nil,
				&actions.ExportAsset{
					To:          rsender,
					Asset:       newAsset,
					Value:       100,
					Return:      false,
					Destination: ids.GenerateTestID(),
				},
				otherFactory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}generated transaction:{{/}} %s\n", txID)

			// Broadcast and wait for transaction
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, err := instancesB[0].cli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeFalse())

			// Confirm balances are unchanged
			newOtherBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(newOtherBalance).Should(gomega.Equal(sendAmount))
		})

		ginkgo.By("submitting first (2000) return export action on destination", func() {
			bIDA, err := ids.FromString(blockchainIDA)
			gomega.Ω(err).Should(gomega.BeNil())
			newAsset := actions.ImportedAssetID(ids.Empty, bIDA)
			submit, tx, _, err := instancesB[0].cli.GenerateTransaction(
				context.Background(),
				nil,
				&actions.ExportAsset{
					To:          rsender,
					Asset:       newAsset,
					Value:       2000,
					Return:      true,
					Destination: source,
					Reward:      100,
				},
				otherFactory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			txID = tx.ID()
			hutils.Outf("{{yellow}}generated transaction:{{/}} %s\n", txID)

			// Broadcast and wait for transaction
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, err := instancesB[0].cli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp export transaction{{/}}\n")

			// Check balances and asset info
			amount, err := instancesB[0].cli.Loan(context.Background(), newAsset, source)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(amount).Should(gomega.Equal(uint64(0)))
			otherBalance, err := instancesB[0].cli.Balance(context.Background(), aother, newAsset)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(otherBalance).Should(gomega.Equal(uint64(2900)))
			exists, metadata, supply, owner, warp, err := instancesB[0].cli.Asset(
				context.Background(),
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(exists).Should(gomega.BeTrue())
			gomega.Ω(metadata).Should(gomega.Equal(actions.ImportedAssetMetadata(ids.Empty, bIDA)))
			gomega.Ω(supply).Should(gomega.Equal(uint64(2900)))
			gomega.Ω(owner).Should(gomega.Equal(utils.Address(crypto.EmptyPublicKey)))
			gomega.Ω(warp).Should(gomega.BeTrue())
		})

		ginkgo.By("submitting first import action on source", func() {
			bIDA, err := ids.FromString(blockchainIDA)
			gomega.Ω(err).Should(gomega.BeNil())
			newAsset := actions.ImportedAssetID(ids.Empty, bIDA)
			nativeOtherBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newOtherBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(newOtherBalance).Should(gomega.Equal(uint64(0)))
			nativeSenderBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newSenderBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				sender,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(newSenderBalance).Should(gomega.Equal(uint64(0)))

			var (
				msg                     *warp.Message
				subnetWeight, sigWeight uint64
			)
			for {
				msg, subnetWeight, sigWeight, err = instancesB[0].cli.GenerateAggregateWarpSignature(
					context.Background(),
					txID,
				)
				if sigWeight == subnetWeight && err == nil {
					break
				}
				if err == nil {
					hutils.Outf(
						"{{yellow}}waiting for signature weight:{{/}} %d {{yellow}}observed:{{/}} %d\n",
						subnetWeight,
						sigWeight,
					)
				} else {
					hutils.Outf("{{red}}found error:{{/}} %v\n", err)
				}
				time.Sleep(1 * time.Second)
			}
			hutils.Outf(
				"{{green}}fetched signature weight:{{/}} %d {{green}}total weight:{{/}} %d\n",
				sigWeight,
				subnetWeight,
			)
			gomega.Ω(subnetWeight).Should(gomega.Equal(sigWeight))

			submit, tx, fees, err := instancesA[0].cli.GenerateTransaction(
				context.Background(),
				msg,
				&actions.ImportAsset{},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			txID = tx.ID()
			hutils.Outf("{{yellow}}generated transaction:{{/}} %s\n", txID)
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, err := instancesA[0].cli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp import transaction{{/}}\n")

			// Check balances and loan
			aNativeOtherBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(nativeOtherBalance).Should(gomega.Equal(aNativeOtherBalance))
			aNewOtherBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNewOtherBalance).Should(gomega.Equal(uint64(0)))
			aNativeSenderBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNativeSenderBalance).
				Should(gomega.Equal(nativeSenderBalance - fees + 2000 + 100))
			aNewSenderBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				sender,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNewSenderBalance).Should(gomega.Equal(uint64(0)))
			amount, err := instancesA[0].cli.Loan(context.Background(), ids.Empty, destination)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(amount).Should(gomega.Equal(uint64(2900)))
		})

		ginkgo.By("submitting second (2900) return export action on destination", func() {
			bIDA, err := ids.FromString(blockchainIDA)
			gomega.Ω(err).Should(gomega.BeNil())
			newAsset := actions.ImportedAssetID(ids.Empty, bIDA)
			submit, tx, _, err := instancesB[0].cli.GenerateTransaction(
				context.Background(),
				nil,
				&actions.ExportAsset{
					To:          other.PublicKey(),
					Asset:       newAsset,
					Value:       2900,
					Return:      true,
					Destination: source,
				},
				otherFactory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			txID = tx.ID()
			hutils.Outf("{{yellow}}generated transaction:{{/}} %s\n", txID)

			// Broadcast and wait for transaction
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, err := instancesB[0].cli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp export transaction{{/}}\n")

			// Check balances and asset info
			amount, err := instancesB[0].cli.Loan(context.Background(), newAsset, source)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(amount).Should(gomega.Equal(uint64(0)))
			otherBalance, err := instancesB[0].cli.Balance(context.Background(), aother, newAsset)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(otherBalance).Should(gomega.Equal(uint64(0)))
			exists, _, _, _, _, err := instancesB[0].cli.Asset(context.Background(), newAsset)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(exists).Should(gomega.BeFalse())
		})

		ginkgo.By("submitting second import action on source", func() {
			bIDA, err := ids.FromString(blockchainIDA)
			gomega.Ω(err).Should(gomega.BeNil())
			newAsset := actions.ImportedAssetID(ids.Empty, bIDA)
			nativeOtherBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newOtherBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(newOtherBalance).Should(gomega.Equal(uint64(0)))
			nativeSenderBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newSenderBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				sender,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(newSenderBalance).Should(gomega.Equal(uint64(0)))

			var (
				msg                     *warp.Message
				subnetWeight, sigWeight uint64
			)
			for {
				msg, subnetWeight, sigWeight, err = instancesB[0].cli.GenerateAggregateWarpSignature(
					context.Background(),
					txID,
				)
				if sigWeight == subnetWeight && err == nil {
					break
				}
				if err == nil {
					hutils.Outf(
						"{{yellow}}waiting for signature weight:{{/}} %d {{yellow}}observed:{{/}} %d\n",
						subnetWeight,
						sigWeight,
					)
				} else {
					hutils.Outf("{{red}}found error:{{/}} %v\n", err)
				}
				time.Sleep(1 * time.Second)
			}
			hutils.Outf(
				"{{green}}fetched signature weight:{{/}} %d {{green}}total weight:{{/}} %d\n",
				sigWeight,
				subnetWeight,
			)
			gomega.Ω(subnetWeight).Should(gomega.Equal(sigWeight))

			submit, tx, fees, err := instancesA[0].cli.GenerateTransaction(
				context.Background(),
				msg,
				&actions.ImportAsset{},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			txID = tx.ID()
			hutils.Outf("{{yellow}}generated transaction:{{/}} %s\n", txID)
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, err := instancesA[0].cli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp import transaction{{/}}\n")

			// Check balances and loan
			aNativeOtherBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNativeOtherBalance).Should(gomega.Equal(nativeOtherBalance + 2900))
			aNewOtherBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNewOtherBalance).Should(gomega.Equal(uint64(0)))
			aNativeSenderBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNativeSenderBalance).Should(gomega.Equal(nativeSenderBalance - fees))
			aNewSenderBalance, err := instancesA[0].cli.Balance(
				context.Background(),
				sender,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNewSenderBalance).Should(gomega.Equal(uint64(0)))
			amount, err := instancesA[0].cli.Loan(context.Background(), ids.Empty, destination)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(amount).Should(gomega.Equal(uint64(0)))
		})

		ginkgo.By("swaping into destination", func() {
			bIDA, err := ids.FromString(blockchainIDA)
			gomega.Ω(err).Should(gomega.BeNil())
			newAsset := actions.ImportedAssetID(ids.Empty, bIDA)
			submit, tx, _, err := instancesA[0].cli.GenerateTransaction(
				context.Background(),
				nil,
				&actions.ExportAsset{
					To:          other.PublicKey(),
					Asset:       ids.Empty, // becomes newAsset
					Value:       2000,
					Return:      false,
					SwapIn:      100,
					AssetOut:    ids.Empty,
					SwapOut:     200,
					SwapExpiry:  time.Now().Unix() + 10_000,
					Destination: destination,
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			txID = tx.ID()
			hutils.Outf("{{yellow}}generated transaction:{{/}} %s\n", txID)

			// Broadcast and wait for transaction
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, err := instancesA[0].cli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp export transaction{{/}}\n")

			// Record balances on destination
			nativeOtherBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newOtherBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(newOtherBalance).Should(gomega.Equal(uint64(0)))
			nativeSenderBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newSenderBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				sender,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(newSenderBalance).Should(gomega.Equal(uint64(0)))

			var (
				msg                     *warp.Message
				subnetWeight, sigWeight uint64
			)
			for {
				msg, subnetWeight, sigWeight, err = instancesA[0].cli.GenerateAggregateWarpSignature(
					context.Background(),
					txID,
				)
				if sigWeight == subnetWeight && err == nil {
					break
				}
				if err == nil {
					hutils.Outf(
						"{{yellow}}waiting for signature weight:{{/}} %d {{yellow}}observed:{{/}} %d\n",
						subnetWeight,
						sigWeight,
					)
				} else {
					hutils.Outf("{{red}}found error:{{/}} %v\n", err)
				}
				time.Sleep(1 * time.Second)
			}
			hutils.Outf(
				"{{green}}fetched signature weight:{{/}} %d {{green}}total weight:{{/}} %d\n",
				sigWeight,
				subnetWeight,
			)
			gomega.Ω(subnetWeight).Should(gomega.Equal(sigWeight))

			submit, tx, fees, err := instancesB[0].cli.GenerateTransaction(
				context.Background(),
				msg,
				&actions.ImportAsset{
					Fill: true,
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			txID = tx.ID()
			hutils.Outf("{{yellow}}generated transaction:{{/}} %s\n", txID)
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
			ctx, cancel = context.WithTimeout(context.Background(), requestTimeout)
			success, err = instancesB[0].cli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp import transaction{{/}}\n")

			// Check balances following swap
			aNativeOtherBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNativeOtherBalance).Should(gomega.Equal(nativeOtherBalance + 200))
			aNewOtherBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNewOtherBalance).Should(gomega.Equal(uint64(1900)))
			aNativeSenderBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNativeSenderBalance).
				Should(gomega.Equal(nativeSenderBalance - fees - 200))
			aNewSenderBalance, err := instancesB[0].cli.Balance(
				context.Background(),
				sender,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNewSenderBalance).Should(gomega.Equal(uint64(100)))
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
	lastHeight := uint64(0)
	ginkgo.It("supports issuance of 128 blocks", func() {
		for {
			// Generate transaction
			other, err := crypto.GeneratePrivateKey()
			gomega.Ω(err).Should(gomega.BeNil())
			submit, _, _, err := instancesA[count%len(instancesA)].cli.GenerateTransaction(
				context.Background(),
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
			count++
			_, height, _, err := instancesA[0].cli.Accepted(context.Background())
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
		cluster, err := anrCli.AddNode(
			context.Background(),
			"bootstrap",
			execPath,
		)
		gomega.Expect(err).To(gomega.BeNil())
		awaitHealthy(anrCli)

		nodeURI := cluster.ClusterInfo.NodeInfos["bootstrap"].Uri
		uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainIDA)
		hutils.Outf("{{blue}}bootstrap node uri: %s{{/}}\n", uri)
		c := client.New(uri)
		syncClient = c
		instancesA = append(instancesA, instance{
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
			submit, _, _, err := instancesA[count%len(instancesA)].cli.GenerateTransaction(
				context.Background(),
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
			count++
			_, height, _, err := instancesA[0].cli.Accepted(context.Background())
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
		cluster, err := anrCli.AddNode(
			context.Background(),
			"sync",
			execPath,
		)
		gomega.Expect(err).To(gomega.BeNil())

		// Wait some time before broadcasting new txs so that state sync can finsih
		// without updating roots
		//
		// TODO: find a way to communicate this explicitly without sleeping
		time.Sleep(30 * time.Second) // assume state sync takes less than this to complete
		awaitHealthy(anrCli)

		nodeURI := cluster.ClusterInfo.NodeInfos["sync"].Uri
		uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainIDA)
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
				submit, _, _, err := instancesA[count%len(instancesA)].cli.GenerateTransaction(
					context.Background(),
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
				count++
				_, height, _, err := instancesA[0].cli.Accepted(context.Background())
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
		cluster, err := anrCli.AddNode(
			context.Background(),
			"sync_concurrent",
			execPath,
		)
		gomega.Expect(err).To(gomega.BeNil())
		awaitHealthy(anrCli)

		nodeURI := cluster.ClusterInfo.NodeInfos["sync_concurrent"].Uri
		uri := nodeURI + fmt.Sprintf("/ext/bc/%s", blockchainIDA)
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
				nil,
				&actions.Transfer{
					To:    other.PublicKey(),
					Value: sendAmount,
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
			submit, _, _, err := instancesA[0].cli.GenerateTransaction(
				context.Background(),
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
			continue
		}
		return
	}
}

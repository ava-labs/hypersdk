// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package e2e

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/fatih/color"

	ginkgo "github.com/onsi/ginkgo/v2"

	"github.com/onsi/gomega"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/tests/fixture/e2e"
	"github.com/ava-labs/avalanchego/tests/fixture/tmpnet"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/rpc"
	hutils "github.com/ava-labs/hypersdk/utils"
)

const (
	startAmount = uint64(10000000000000000000)
	sendAmount  = uint64(5000)

	healthPollInterval = 10 * time.Second

	subnetAName = "subnet-a"
	subnetBName = "subnet-b"

	// TODO(marun) Generate a key instead of hard-coding
	fundedAddress = "token1qrzvk4zlwj9zsacqgtufx7zvapd3quufqpxk5rsdd4633m4wz2fdj73w34s"
)

func TestE2e(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "tokenvm e2e test suites")
}

var (
	flagVars *e2e.FlagVars
	env      *e2e.TestEnvironment

	requestTimeout time.Duration

	subnetA *tmpnet.Subnet
	subnetB *tmpnet.Subnet

	blockchainIDA ids.ID
	blockchainIDB ids.ID

	subnetsToTrack string

	numValidators uint
)

func init() {
	// Configures flags used to configure tmpnet (via SynchronizedBeforeSuite)
	flagVars = e2e.RegisterFlags()

	flag.DurationVar(
		&requestTimeout,
		"request-timeout",
		120*time.Second,
		"timeout for transaction issuance and confirmation",
	)

	flag.UintVar(
		&numValidators,
		"num-validators",
		5,
		"number of validators per blockchain",
	)
}

var _ = ginkgo.BeforeSuite(func() {
	gomega.Expect(numValidators).Should(gomega.BeNumerically(">", 0))

	subnetSize := int(numValidators)
	nodes := newTmpnetNodes(subnetSize * 2)

	env = e2e.NewTestEnvironment(
		flagVars,
		newTmpnetNetwork(
			nodes,
			// Use half of the valiators for each subnet
			newTmpnetSubnet(subnetAName, nodes[0:subnetSize]...),
			newTmpnetSubnet(subnetBName, nodes[subnetSize:len(nodes)]...),
		),
	)

	network := env.GetNetwork()
	subnetA := network.GetSubnet(subnetAName)
	blockchainIDA := subnetA.Chains[0].ChainID
	subnetB := network.GetSubnet(subnetBName)
	blockchainIDB := subnetB.Chains[0].ChainID
	subnetsToTrack = fmt.Sprintf("%s,%s", subnetA.SubnetID, subnetB.SubnetID)

	// TODO(marun) Support 'single' mode

	// TODO(marun) Improve the DX around discovering the URIs of subnet validators
	subnetAValidatorIDs := &set.Set[ids.NodeID]{}
	subnetAValidatorIDs.Add(subnetA.ValidatorIDs...)
	subnetBValidatorIDs := &set.Set[ids.NodeID]{}
	subnetBValidatorIDs.Add(subnetB.ValidatorIDs...)

	newInstance := func(nodeURI tmpnet.NodeURI, chainID ids.ID) instance {
		tvmURI := fmt.Sprintf("%s/ext/bc/%s", nodeURI.URI, chainID)
		return instance{
			nodeID: nodeURI.NodeID,
			uri:    nodeURI.URI,
			cli:    rpc.NewJSONRPCClient(tvmURI),
			tcli:   trpc.NewJSONRPCClient(tvmURI, network.Genesis.NetworkID, chainID),
		}
	}

	for _, nodeURI := range network.GetNodeURIs() {
		if subnetAValidatorIDs.Contains(nodeURI.NodeID) {
			instancesA = append(instancesA, newInstance(nodeURI, blockchainIDA))
		}
		if subnetBValidatorIDs.Contains(nodeURI.NodeID) {
			instancesB = append(instancesB, newInstance(nodeURI, blockchainIDB))
		}
	}

	// Load default pk
	privBytes, err := codec.LoadHex(
		"323b1d8f4eed5f0da9da93071b034f2dce9d2d22692c172f3cb252a64ddfafd01b057de320297c29ad0c1f589ea216869cf1938d88c9fbd70d6748323dbf2fa7", //nolint:lll
		ed25519.PrivateKeyLen,
	)
	gomega.Ω(err).Should(gomega.BeNil())
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

	instancesA []instance
	instancesB []instance
)

type instance struct {
	nodeID ids.NodeID
	uri    string
	cli    *rpc.JSONRPCClient
	tcli   *trpc.JSONRPCClient
}

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
			_, _, chainID, err := cli.Network(context.Background())
			gomega.Ω(chainID).ShouldNot(gomega.Equal(ids.Empty))
			gomega.Ω(err).Should(gomega.BeNil())
		}
	})

	ginkgo.It("can get network B", func() {
		for _, inst := range instancesB {
			cli := inst.cli
			_, _, chainID, err := cli.Network(context.Background())
			gomega.Ω(chainID).ShouldNot(gomega.Equal(ids.Empty))
			gomega.Ω(err).Should(gomega.BeNil())
		}
	})
})

var _ = ginkgo.Describe("[Test]", func() {
	require := require.New(ginkgo.GinkgoT())

	ginkgo.It("maru transfer in a single node (raw)", func() {
		hutils.Outf(" targeting %s\n", instancesA[0].nodeID)

		ginkgo.By("checking that sufficient balance is present")
		minBalance := sendAmount * 2 // Arbitrary amount greater than the send amount to account for tx fees
		nativeBalance, err := instancesA[0].tcli.Balance(context.TODO(), sender, ids.Empty)
		require.NoError(err)
		require.Greater(nativeBalance, minBalance)

		other, err := ed25519.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		aother := auth.NewED25519Address(other.PublicKey())

		ginkgo.By("issue Transfer to the first node", func() {
			// Generate transaction
			parser, err := instancesA[0].tcli.Parser(context.TODO())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, tx, maxFee, err := instancesA[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				nil,
				&actions.Transfer{
					To:    aother,
					Value: sendAmount,
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}generated transaction %q{{/}}\n", tx.ID())

			// Broadcast and wait for transaction
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, fee, err := instancesA[0].tcli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found transaction{{/}}\n")

			// Check sender balance
			balance, err := instancesA[0].tcli.Balance(context.Background(), sender, ids.Empty)
			gomega.Ω(err).Should(gomega.BeNil())
			hutils.Outf(
				"{{yellow}}start=%d fee=%d send=%d balance=%d{{/}}\n",
				startAmount,
				maxFee,
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
				balance, err := inst.tcli.Balance(context.Background(), codec.MustAddressBech32(consts.HRP, aother), ids.Empty)
				gomega.Ω(err).Should(gomega.BeNil())
				gomega.Ω(balance).Should(gomega.Equal(sendAmount))
			}
		})
	})

	ginkgo.It("performs a warp transfer of the native asset", func() {
		other, err := ed25519.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		aother := codec.MustAddressBech32(consts.HRP, auth.NewED25519Address(other.PublicKey()))
		source := blockchainIDA
		destination := blockchainIDB
		otherFactory := auth.NewED25519Factory(other)

		var txID ids.ID
		ginkgo.By("submitting an export action on source", func() {
			otherBalance, err := instancesA[0].tcli.Balance(context.Background(), aother, ids.Empty)
			gomega.Ω(err).Should(gomega.BeNil())
			senderBalance, err := instancesA[0].tcli.Balance(context.Background(), sender, ids.Empty)
			gomega.Ω(err).Should(gomega.BeNil())

			parser, err := instancesA[0].tcli.Parser(context.TODO())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, tx, _, err := instancesA[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				nil,
				&actions.ExportAsset{
					To:          auth.NewED25519Address(other.PublicKey()),
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
			success, fee, err := instancesA[0].tcli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp export transaction{{/}}\n")

			// Check loans and balances
			amount, err := instancesA[0].tcli.Loan(context.Background(), ids.Empty, destination)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(amount).Should(gomega.Equal(sendAmount))
			aotherBalance, err := instancesA[0].tcli.Balance(context.Background(), aother, ids.Empty)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(otherBalance).Should(gomega.Equal(aotherBalance))
			asenderBalance, err := instancesA[0].tcli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(asenderBalance).Should(gomega.Equal(senderBalance - sendAmount - fee))
		})

		ginkgo.By("fund other account with native", func() {
			parser, err := instancesB[0].tcli.Parser(context.TODO())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, tx, _, err := instancesB[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				nil,
				&actions.Transfer{
					To:    auth.NewED25519Address(other.PublicKey()),
					Asset: ids.Empty,
					Value: 500_000_000,
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			txID := tx.ID()
			hutils.Outf("{{yellow}}generated transaction:{{/}} %s\n", txID)

			// Broadcast transaction (wait for after all broadcast)
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			hutils.Outf("{{yellow}}submitted transaction{{/}}\n")

			// Confirm transaction is accepted
			ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
			success, _, err := instancesB[0].tcli.WaitForTransaction(ctx, txID)
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found transaction %s on B{{/}}\n", txID)
		})

		ginkgo.By("submitting an import action on destination", func() {
			newAsset := actions.ImportedAssetID(ids.Empty, blockchainIDA)
			nativeOtherBalance, err := instancesB[0].tcli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newOtherBalance, err := instancesB[0].tcli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(newOtherBalance).Should(gomega.Equal(uint64(0)))
			nativeSenderBalance, err := instancesB[0].tcli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newSenderBalance, err := instancesB[0].tcli.Balance(
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

			parser, err := instancesB[0].tcli.Parser(context.TODO())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, tx, _, err := instancesB[0].cli.GenerateTransaction(
				context.Background(),
				parser,
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
			success, fee, err := instancesB[0].tcli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp import transaction{{/}}\n")

			// Check asset info and balance
			aNativeOtherBalance, err := instancesB[0].tcli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(nativeOtherBalance).Should(gomega.Equal(aNativeOtherBalance))
			aNewOtherBalance, err := instancesB[0].tcli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNewOtherBalance).Should(gomega.Equal(sendAmount))
			aNativeSenderBalance, err := instancesB[0].tcli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNativeSenderBalance).Should(gomega.Equal(nativeSenderBalance - fee))
			aNewSenderBalance, err := instancesB[0].tcli.Balance(
				context.Background(),
				sender,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNewSenderBalance).Should(gomega.Equal(uint64(0)))
			exists, symbol, decimals, metadata, supply, owner, warp, err := instancesB[0].tcli.Asset(context.Background(), newAsset, false)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(exists).Should(gomega.BeTrue())
			gomega.Ω(string(symbol)).Should(gomega.Equal(consts.Symbol))
			gomega.Ω(decimals).Should(gomega.Equal(uint8(consts.Decimals)))
			gomega.Ω(metadata).Should(gomega.Equal(actions.ImportedAssetMetadata(ids.Empty, blockchainIDA)))
			gomega.Ω(supply).Should(gomega.Equal(sendAmount))
			gomega.Ω(owner).Should(gomega.Equal(codec.MustAddressBech32(consts.HRP, codec.EmptyAddress)))
			gomega.Ω(warp).Should(gomega.BeTrue())
		})

		ginkgo.By("submitting an invalid export action to new destination", func() {
			newAsset := actions.ImportedAssetID(ids.Empty, blockchainIDA)
			parser, err := instancesB[0].tcli.Parser(context.TODO())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, tx, _, err := instancesB[0].cli.GenerateTransaction(
				context.Background(),
				parser,
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
			success, _, err := instancesB[0].tcli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeFalse())

			// Confirm balances are unchanged
			newOtherBalance, err := instancesB[0].tcli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(newOtherBalance).Should(gomega.Equal(sendAmount))
		})

		ginkgo.By("submitting first (2000) return export action on destination", func() {
			newAsset := actions.ImportedAssetID(ids.Empty, blockchainIDA)
			parser, err := instancesB[0].tcli.Parser(context.TODO())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, tx, _, err := instancesB[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				nil,
				&actions.ExportAsset{
					To:          rsender,
					Asset:       newAsset,
					Value:       2000,
					Return:      true,
					Destination: blockchainIDA,
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
			success, _, err := instancesB[0].tcli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp export transaction{{/}}\n")

			// Check balances and asset info
			amount, err := instancesB[0].tcli.Loan(context.Background(), newAsset, source)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(amount).Should(gomega.Equal(uint64(0)))
			otherBalance, err := instancesB[0].tcli.Balance(context.Background(), aother, newAsset)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(otherBalance).Should(gomega.Equal(uint64(2900)))
			exists, symbol, decimals, metadata, supply, owner, warp, err := instancesB[0].tcli.Asset(context.Background(), newAsset, false)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(exists).Should(gomega.BeTrue())
			gomega.Ω(string(symbol)).Should(gomega.Equal(consts.Symbol))
			gomega.Ω(decimals).Should(gomega.Equal(uint8(consts.Decimals)))
			gomega.Ω(metadata).Should(gomega.Equal(actions.ImportedAssetMetadata(ids.Empty, blockchainIDA)))
			gomega.Ω(supply).Should(gomega.Equal(uint64(2900)))
			gomega.Ω(owner).Should(gomega.Equal(codec.MustAddressBech32(consts.HRP, codec.EmptyAddress)))
			gomega.Ω(warp).Should(gomega.BeTrue())
		})

		ginkgo.By("submitting first import action on source", func() {
			newAsset := actions.ImportedAssetID(ids.Empty, blockchainIDA)
			nativeOtherBalance, err := instancesA[0].tcli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newOtherBalance, err := instancesA[0].tcli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(newOtherBalance).Should(gomega.Equal(uint64(0)))
			nativeSenderBalance, err := instancesA[0].tcli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newSenderBalance, err := instancesA[0].tcli.Balance(
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

			parser, err := instancesA[0].tcli.Parser(context.TODO())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, tx, _, err := instancesA[0].cli.GenerateTransaction(
				context.Background(),
				parser,
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
			success, fee, err := instancesA[0].tcli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp import transaction{{/}}\n")

			// Check balances and loan
			aNativeOtherBalance, err := instancesA[0].tcli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(nativeOtherBalance).Should(gomega.Equal(aNativeOtherBalance))
			aNewOtherBalance, err := instancesA[0].tcli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNewOtherBalance).Should(gomega.Equal(uint64(0)))
			aNativeSenderBalance, err := instancesA[0].tcli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNativeSenderBalance).
				Should(gomega.Equal(nativeSenderBalance - fee + 2000 + 100))
			aNewSenderBalance, err := instancesA[0].tcli.Balance(
				context.Background(),
				sender,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNewSenderBalance).Should(gomega.Equal(uint64(0)))
			amount, err := instancesA[0].tcli.Loan(context.Background(), ids.Empty, destination)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(amount).Should(gomega.Equal(uint64(2900)))
		})

		ginkgo.By("submitting second (2900) return export action on destination", func() {
			newAsset := actions.ImportedAssetID(ids.Empty, blockchainIDA)
			parser, err := instancesB[0].tcli.Parser(context.TODO())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, tx, _, err := instancesB[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				nil,
				&actions.ExportAsset{
					To:          auth.NewED25519Address(other.PublicKey()),
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
			success, _, err := instancesB[0].tcli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp export transaction{{/}}\n")

			// Check balances and asset info
			amount, err := instancesB[0].tcli.Loan(context.Background(), newAsset, source)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(amount).Should(gomega.Equal(uint64(0)))
			otherBalance, err := instancesB[0].tcli.Balance(context.Background(), aother, newAsset)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(otherBalance).Should(gomega.Equal(uint64(0)))
			exists, _, _, _, _, _, _, err := instancesB[0].tcli.Asset(context.Background(), newAsset, false)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(exists).Should(gomega.BeFalse())
		})

		ginkgo.By("submitting second import action on source", func() {
			newAsset := actions.ImportedAssetID(ids.Empty, blockchainIDA)
			nativeOtherBalance, err := instancesA[0].tcli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newOtherBalance, err := instancesA[0].tcli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(newOtherBalance).Should(gomega.Equal(uint64(0)))
			nativeSenderBalance, err := instancesA[0].tcli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newSenderBalance, err := instancesA[0].tcli.Balance(
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

			parser, err := instancesA[0].tcli.Parser(context.TODO())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, tx, _, err := instancesA[0].cli.GenerateTransaction(
				context.Background(),
				parser,
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
			success, fee, err := instancesA[0].tcli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp import transaction{{/}}\n")

			// Check balances and loan
			aNativeOtherBalance, err := instancesA[0].tcli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNativeOtherBalance).Should(gomega.Equal(nativeOtherBalance + 2900))
			aNewOtherBalance, err := instancesA[0].tcli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNewOtherBalance).Should(gomega.Equal(uint64(0)))
			aNativeSenderBalance, err := instancesA[0].tcli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNativeSenderBalance).Should(gomega.Equal(nativeSenderBalance - fee))
			aNewSenderBalance, err := instancesA[0].tcli.Balance(
				context.Background(),
				sender,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNewSenderBalance).Should(gomega.Equal(uint64(0)))
			amount, err := instancesA[0].tcli.Loan(context.Background(), ids.Empty, destination)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(amount).Should(gomega.Equal(uint64(0)))
		})

		ginkgo.By("swaping into destination", func() {
			newAsset := actions.ImportedAssetID(ids.Empty, blockchainIDA)
			parser, err := instancesA[0].tcli.Parser(context.TODO())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, tx, _, err := instancesA[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				nil,
				&actions.ExportAsset{
					To:          auth.NewED25519Address(other.PublicKey()),
					Asset:       ids.Empty, // becomes newAsset
					Value:       2000,
					Return:      false,
					SwapIn:      100,
					AssetOut:    ids.Empty,
					SwapOut:     200,
					SwapExpiry:  time.Now().UnixMilli() + 100_000,
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
			success, _, err := instancesA[0].tcli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp export transaction{{/}}\n")

			// Record balances on destination
			nativeOtherBalance, err := instancesB[0].tcli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newOtherBalance, err := instancesB[0].tcli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(newOtherBalance).Should(gomega.Equal(uint64(0)))
			nativeSenderBalance, err := instancesB[0].tcli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			newSenderBalance, err := instancesB[0].tcli.Balance(
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

			parser, err = instancesB[0].tcli.Parser(context.TODO())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, tx, _, err = instancesB[0].cli.GenerateTransaction(
				context.Background(),
				parser,
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
			success, fee, err := instancesB[0].tcli.WaitForTransaction(ctx, tx.ID())
			cancel()
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(success).Should(gomega.BeTrue())
			hutils.Outf("{{yellow}}found warp import transaction{{/}}\n")

			// Check balances following swap
			aNativeOtherBalance, err := instancesB[0].tcli.Balance(
				context.Background(),
				aother,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNativeOtherBalance).Should(gomega.Equal(nativeOtherBalance + 200))
			aNewOtherBalance, err := instancesB[0].tcli.Balance(
				context.Background(),
				aother,
				newAsset,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNewOtherBalance).Should(gomega.Equal(uint64(1900)))
			aNativeSenderBalance, err := instancesB[0].tcli.Balance(
				context.Background(),
				sender,
				ids.Empty,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(aNativeSenderBalance).
				Should(gomega.Equal(nativeSenderBalance - fee - 200))
			aNewSenderBalance, err := instancesB[0].tcli.Balance(
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

	// Create blocks before bootstrapping starts
	count := 0
	ginkgo.It("supports issuance of 128 blocks", func() {
		count += generateBlocks(context.Background(), count, 128, instancesA, true)
	})

	// Ensure bootstrapping works
	var syncClient *rpc.JSONRPCClient
	var tsyncClient *trpc.JSONRPCClient
	ginkgo.It("can bootstrap a new node", func() {
		network := env.GetNetwork()
		node := e2e.AddEphemeralNode(network, tmpnet.FlagsMap{
			config.TrackSubnetsKey: subnetsToTrack,
		})
		e2e.WaitForHealthy(node)

		uri := node.URI + fmt.Sprintf("/ext/bc/%s", blockchainIDA)
		hutils.Outf("{{blue}}bootstrap node uri: %s{{/}}\n", uri)
		c := rpc.NewJSONRPCClient(uri)
		syncClient = c
		networkID, _, _, err := syncClient.Network(e2e.DefaultContext())
		gomega.Expect(err).Should(gomega.BeNil())
		tc := trpc.NewJSONRPCClient(uri, networkID, blockchainIDA)
		tsyncClient = tc
		instancesA = append(instancesA, instance{
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
		count += generateBlocks(context.Background(), count, 1024, instancesA, true)
		// TODO: verify all roots are equal
	})

	var syncNode *tmpnet.Node
	ginkgo.It("can state sync a new node when no new blocks are being produced", func() {
		network := env.GetNetwork()
		syncNode = e2e.AddEphemeralNode(network, tmpnet.FlagsMap{
			config.TrackSubnetsKey: subnetsToTrack,
		})
		e2e.WaitForHealthy(syncNode)

		uri := syncNode.URI + fmt.Sprintf("/ext/bc/%s", blockchainIDA)
		hutils.Outf("{{blue}}sync node uri: %s{{/}}\n", uri)
		syncClient = rpc.NewJSONRPCClient(uri)
		networkID, _, _, err := syncClient.Network(context.TODO())
		gomega.Expect(err).To(gomega.BeNil())
		tsyncClient = trpc.NewJSONRPCClient(uri, networkID, blockchainIDA)
	})

	ginkgo.It("accepts transaction after state sync", func() {
		acceptTransaction(syncClient, tsyncClient)
	})

	ginkgo.It("can stop a node", func() {
		err := syncNode.Stop(e2e.DefaultContext())
		gomega.Expect(err).To(gomega.BeNil())

		ok, err := syncClient.Ping(context.Background())
		gomega.Ω(ok).Should(gomega.BeFalse())
		gomega.Ω(err).Should(gomega.HaveOccurred())
	})

	ginkgo.It("supports issuance of 256 more blocks", func() {
		count += generateBlocks(context.Background(), count, 256, instancesA, true)
		// TODO: verify all roots are equal
	})

	ginkgo.It("can re-sync the restarted node", func() {
		network := env.GetNetwork()
		err := network.StartNode(e2e.DefaultContext(), ginkgo.GinkgoWriter, syncNode)
		gomega.Expect(err).To(gomega.BeNil())
		e2e.WaitForHealthy(syncNode)
	})

	ginkgo.It("accepts transaction after restarted node state sync", func() {
		acceptTransaction(syncClient, tsyncClient)
	})

	ginkgo.It("state sync while broadcasting transactions", func() {
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			// Recover failure if exits
			defer ginkgo.GinkgoRecover()

			count += generateBlocks(ctx, count, 0, instancesA, false)
		}()

		// Give time for transactions to start processing
		time.Sleep(5 * time.Second)

		// Start syncing node
		network := env.GetNetwork()
		node := e2e.AddEphemeralNode(network, tmpnet.FlagsMap{
			config.TrackSubnetsKey: subnetsToTrack,
		})
		e2e.WaitForHealthy(node)

		uri := node.URI + fmt.Sprintf("/ext/bc/%s", blockchainIDA)
		hutils.Outf("{{blue}}sync node uri: %s{{/}}\n", uri)
		syncClient = rpc.NewJSONRPCClient(uri)
		networkID, _, _, err := syncClient.Network(context.TODO())
		gomega.Expect(err).To(gomega.BeNil())
		tsyncClient = trpc.NewJSONRPCClient(uri, networkID, blockchainIDA)
		cancel()
	})

	ginkgo.It("accepts transaction after state sync concurrent", func() {
		acceptTransaction(syncClient, tsyncClient)
	})

	// TODO: restart all nodes (crisis simulation)
})

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
	parser, err := instances[0].tcli.Parser(context.Background())
	gomega.Ω(err).Should(gomega.BeNil())
	var targetHeight uint64
	if blocksToGenerate != 0 {
		targetHeight = lastHeight + blocksToGenerate
	}
	for ctx.Err() == nil {
		// Generate transaction
		other, err := ed25519.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[cumulativeTxs%len(instances)].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.Transfer{
				To:    auth.NewED25519Address(other.PublicKey()),
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

func acceptTransaction(cli *rpc.JSONRPCClient, tcli *trpc.JSONRPCClient) {
	parser, err := tcli.Parser(context.Background())
	gomega.Ω(err).Should(gomega.BeNil())
	for {
		// Generate transaction
		other, err := ed25519.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		unitPrices, err := cli.UnitPrices(context.Background(), false)
		gomega.Ω(err).Should(gomega.BeNil())
		submit, tx, maxFee, err := cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.Transfer{
				To:    auth.NewED25519Address(other.PublicKey()),
				Value: sendAmount,
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		hutils.Outf("{{yellow}}generated transaction{{/}} prices: %+v maxFee: %d\n", unitPrices, maxFee)

		// Broadcast and wait for transaction
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		hutils.Outf("{{yellow}}submitted transaction{{/}}\n")
		ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
		success, _, err := tcli.WaitForTransaction(ctx, tx.ID())
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

func newTmpnetNodes(count int) []*tmpnet.Node {
	nodes := make([]*tmpnet.Node, count)
	for i := range nodes {
		node := tmpnet.NewNode("")
		node.EnsureKeys()
		nodes[i] = node
	}
	return nodes
}

func newTmpnetNetwork(nodes []*tmpnet.Node, subnets ...*tmpnet.Subnet) *tmpnet.Network {
	return &tmpnet.Network{
		DefaultFlags: tmpnet.FlagsMap{
			config.ProposerVMUseCurrentHeightKey: true,
		},
		Nodes:   nodes,
		Subnets: subnets,
	}
}

// Create the configuration that will enable creation and access to a
// subnet created on a temporary network.
func newTmpnetSubnet(name string, nodes ...*tmpnet.Node) *tmpnet.Subnet {
	if len(nodes) == 0 {
		panic("a subnet must be validated by at least one node")
	}

	validatorIDs := make([]ids.NodeID, len(nodes))
	for i, node := range nodes {
		validatorIDs[i] = node.NodeID
	}

	// TODO(marun) Support provision of a genesis file and/or configuring genesis values via flags
	chainGenesis := genesis.Default()
	// TODO(marun) Values copied from run.sh. Are these necessary for testing or would the defaults work?
	chainGenesis.WindowTargetUnits = fees.Dimensions{40000000, 450000, 450000, 450000, 450000}
	chainGenesis.MaxBlockUnits = fees.Dimensions{1800000, 15000, 15000, 2500, 15000}
	chainGenesis.CustomAllocation = []*genesis.CustomAllocation{
		{
			Address: fundedAddress,
			Balance: 10000000000000000000,
		},
	}

	// Converting the genesis to a FlagsMap to allow embedding in
	// tmpnet configuration without having to restort to using bytes.
	flagMapGenesis, err := toFlagsMap(chainGenesis)
	if err != nil {
		panic(fmt.Sprintf("failed to convert genesis to flag map: %v", err))
	}

	// Only supply non-default values
	vmConfig := tmpnet.FlagsMap{
		"mempoolSize":               10000000,
		"mempoolSponsorSize":        10000000,
		"mempoolExemptSponsors":     []string{fundedAddress},
		"authVerificationCores":     2,
		"rootGenerationCores":       2,
		"transactionExecutionCores": 2,
		"storeTransactions":         false,
		"streamingBacklogSize":      10000000,
		"trackedPairs":              []string{"*"},
		"logLevel":                  "debug",
		// TODO(marun) Configure this to use the network path?
		// "continuousProfilerDir": fmt.Sprintf("%s/tokenvm-e2e-profiles/*", network.Dir),
	}

	return &tmpnet.Subnet{
		Name: name,
		Config: tmpnet.FlagsMap{
			"proposerMinBlockDelay":       0,
			"proposerNumHistoricalBlocks": 50000,
		},
		Chains: []*tmpnet.Chain{
			{
				VMID:    consts.ID,
				Genesis: flagMapGenesis,
				Config:  vmConfig,
			},
		},
		ValidatorIDs: validatorIDs,
	}
}

// TODO(marun) Add this as a tmpnet utility function
func toFlagsMap(v interface{}) (tmpnet.FlagsMap, error) {
	bytes, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	flagsMap := tmpnet.FlagsMap{}
	err = json.Unmarshal(bytes, &flagsMap)
	if err != nil {
		return nil, err
	}
	return flagsMap, nil
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	avago_version "github.com/ava-labs/avalanchego/version"
	"github.com/fatih/color"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/crypto"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/vm"

	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	"github.com/ava-labs/hypersdk/examples/tokenvm/client"
	"github.com/ava-labs/hypersdk/examples/tokenvm/controller"
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
	"github.com/ava-labs/hypersdk/examples/tokenvm/utils"
)

const transferTxFee = 400 /* base fee */ + 72 /* transfer fee */

var (
	logFactory logging.Factory
	log        logging.Logger
)

func init() {
	logFactory = logging.NewFactory(logging.Config{
		DisplayLevel: logging.Debug,
	})
	l, err := logFactory.Make("main")
	if err != nil {
		panic(err)
	}
	log = l
}

func TestIntegration(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecs(t, "tokenvm integration test suites")
}

var (
	requestTimeout time.Duration
	vms            int
	minPrice       int64
)

func init() {
	flag.DurationVar(
		&requestTimeout,
		"request-timeout",
		120*time.Second,
		"timeout for transaction issuance and confirmation",
	)
	flag.IntVar(
		&vms,
		"vms",
		3,
		"number of VMs to create",
	)
	flag.Int64Var(
		&minPrice,
		"min-price",
		-1,
		"minimum price",
	)
}

var (
	priv    crypto.PrivateKey
	factory *auth.ED25519Factory
	rsender crypto.PublicKey
	sender  string

	priv2    crypto.PrivateKey
	factory2 *auth.ED25519Factory
	rsender2 crypto.PublicKey
	sender2  string

	asset1 ids.ID
	asset2 ids.ID
	asset3 ids.ID

	// when used with embedded VMs
	genesisBytes []byte
	instances    []instance

	gen *genesis.Genesis
)

type instance struct {
	chainID    ids.ID
	nodeID     ids.NodeID
	vm         *vm.VM
	toEngine   chan common.Message
	httpServer *httptest.Server
	cli        *client.Client // clients for embedded VMs
}

var _ = ginkgo.BeforeSuite(func() {
	gomega.Ω(vms).Should(gomega.BeNumerically(">", 1))

	var err error
	priv, err = crypto.GeneratePrivateKey()
	gomega.Ω(err).Should(gomega.BeNil())
	factory = auth.NewED25519Factory(priv)
	rsender = priv.PublicKey()
	sender = utils.Address(rsender)
	log.Debug(
		"generated key",
		zap.String("addr", sender),
		zap.String("pk", hex.EncodeToString(priv[:])),
	)

	priv2, err = crypto.GeneratePrivateKey()
	gomega.Ω(err).Should(gomega.BeNil())
	factory2 = auth.NewED25519Factory(priv2)
	rsender2 = priv2.PublicKey()
	sender2 = utils.Address(rsender2)
	log.Debug(
		"generated key",
		zap.String("addr", sender2),
		zap.String("pk", hex.EncodeToString(priv2[:])),
	)

	asset1 = hutils.ToID([]byte("1"))
	asset2 = hutils.ToID([]byte("2"))
	asset3 = hutils.ToID([]byte("3"))
	pair := actions.PairID(asset2, asset3)

	// create embedded VMs
	instances = make([]instance, vms)

	gen = genesis.Default()
	if minPrice >= 0 {
		gen.MinUnitPrice = uint64(minPrice)
	}
	gen.WindowTargetBlocks = 1_000_000 // deactivate block fee
	gen.CustomAllocation = []*genesis.CustomAllocation{
		{
			Address: sender,
			Balance: 10_000_000,
		},
	}
	genesisBytes, err = json.Marshal(gen)
	gomega.Ω(err).Should(gomega.BeNil())

	networkID := uint32(1)
	subnetID := ids.GenerateTestID()
	chainID := ids.GenerateTestID()

	app := &appSender{}
	for i := range instances {
		nodeID := ids.GenerateTestNodeID()
		l, err := logFactory.Make(nodeID.String())
		gomega.Ω(err).Should(gomega.BeNil())
		dname, err := os.MkdirTemp("", fmt.Sprintf("%s-chainData", nodeID.String()))
		gomega.Ω(err).Should(gomega.BeNil())
		snowCtx := &snow.Context{
			NetworkID:    networkID,
			SubnetID:     subnetID,
			ChainID:      chainID,
			NodeID:       nodeID,
			Log:          l,
			ChainDataDir: dname,
			Metrics:      metrics.NewOptionalGatherer(),
		}

		toEngine := make(chan common.Message, 1)
		db := manager.NewMemDB(avago_version.CurrentDatabase)

		v := controller.New()
		err = v.Initialize(
			context.TODO(),
			snowCtx,
			db,
			genesisBytes,
			nil,
			[]byte(
				fmt.Sprintf(
					`{"parallelism":3, "testMode":true, "logLevel":"debug", "trackedPairs":["%s"]}`,
					pair,
				),
			),
			toEngine,
			nil,
			app,
		)
		gomega.Ω(err).Should(gomega.BeNil())

		var hd map[string]*common.HTTPHandler
		hd, err = v.CreateHandlers(context.TODO())
		gomega.Ω(err).Should(gomega.BeNil())

		httpServer := httptest.NewServer(hd[vm.Endpoint].Handler)
		instances[i] = instance{
			chainID:    snowCtx.ChainID,
			nodeID:     snowCtx.NodeID,
			vm:         v,
			toEngine:   toEngine,
			httpServer: httpServer,
			cli:        client.New(httpServer.URL),
		}

		// Force sync ready (to mimic bootstrapping from genesis)
		v.ForceReady()
	}

	// Verify genesis allocations loaded correctly (do here otherwise test may
	// check during and it will be inaccurate)
	for _, inst := range instances {
		cli := inst.cli
		g, err := cli.Genesis(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())

		for _, alloc := range g.CustomAllocation {
			balance, err := cli.Balance(context.Background(), alloc.Address, ids.Empty)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(balance).Should(gomega.Equal(alloc.Balance))
		}
	}

	app.instances = instances
	color.Blue("created %d VMs", vms)
})

var _ = ginkgo.AfterSuite(func() {
	for _, iv := range instances {
		iv.httpServer.Close()
		err := iv.vm.Shutdown(context.TODO())
		gomega.Ω(err).Should(gomega.BeNil())
	}
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
			networkID, subnetID, chainID, err := cli.Network(context.Background())
			gomega.Ω(networkID).Should(gomega.Equal(uint32(1)))
			gomega.Ω(subnetID).ShouldNot(gomega.Equal(ids.Empty))
			gomega.Ω(chainID).ShouldNot(gomega.Equal(ids.Empty))
			gomega.Ω(err).Should(gomega.BeNil())
		}
	})
})

var _ = ginkgo.Describe("[Tx Processing]", func() {
	ginkgo.It("get currently accepted block ID", func() {
		for _, inst := range instances {
			cli := inst.cli
			_, _, err := cli.Accepted(context.Background())
			gomega.Ω(err).Should(gomega.BeNil())
		}
	})

	var transferTxRoot *chain.Transaction
	ginkgo.It("Gossip TransferTx to a different node", func() {
		ginkgo.By("issue TransferTx", func() {
			submit, transferTx, _, err := instances[0].cli.GenerateTransaction(
				context.Background(),
				&actions.Transfer{
					To:    rsender2,
					Value: 100_000, // must be more than StateLockup
				},
				factory,
			)
			transferTxRoot = transferTx
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			gomega.Ω(instances[0].vm.Mempool().Len(context.Background())).Should(gomega.Equal(1))
		})

		ginkgo.By("skip duplicate", func() {
			_, err := instances[0].cli.SubmitTx(
				context.Background(),
				transferTxRoot.Bytes(),
			)
			gomega.Ω(err).To(gomega.Not(gomega.BeNil()))
		})

		ginkgo.By("send gossip from node 0 to 1", func() {
			err := instances[0].vm.Gossiper().TriggerGossip(context.TODO())
			gomega.Ω(err).Should(gomega.BeNil())
		})

		ginkgo.By("skip invalid time", func() {
			tx := chain.NewTx(
				&chain.Base{
					ChainID:   instances[0].chainID,
					Timestamp: 0,
					UnitPrice: 1000,
				},
				&actions.Transfer{
					To:    rsender2,
					Value: 110,
				},
			)
			gomega.Ω(tx.Sign(factory)).To(gomega.BeNil())
			actionRegistry, authRegistry := instances[0].vm.Registry()
			sigVerify, err := tx.Init(context.Background(), actionRegistry, authRegistry)
			gomega.Ω(err).To(gomega.BeNil())
			gomega.Ω(sigVerify()).To(gomega.BeNil())
			_, err = instances[0].cli.SubmitTx(
				context.Background(),
				tx.Bytes(),
			)
			gomega.Ω(err).To(gomega.Not(gomega.BeNil()))
		})

		ginkgo.By("skip duplicate (after gossip, which shouldn't clear)", func() {
			_, err := instances[0].cli.SubmitTx(
				context.Background(),
				transferTxRoot.Bytes(),
			)
			gomega.Ω(err).To(gomega.Not(gomega.BeNil()))
		})

		ginkgo.By("receive gossip in the node 1, and signal block build", func() {
			instances[1].vm.Builder().TriggerBuild()
			<-instances[1].toEngine
		})

		ginkgo.By("build block in the node 1", func() {
			ctx := context.TODO()
			blk, err := instances[1].vm.BuildBlock(ctx)
			gomega.Ω(err).To(gomega.BeNil())

			gomega.Ω(blk.Verify(ctx)).To(gomega.BeNil())
			gomega.Ω(blk.Status()).To(gomega.Equal(choices.Processing))

			err = instances[1].vm.SetPreference(ctx, blk.ID())
			gomega.Ω(err).To(gomega.BeNil())

			gomega.Ω(blk.Accept(ctx)).To(gomega.BeNil())
			gomega.Ω(blk.Status()).To(gomega.Equal(choices.Accepted))

			lastAccepted, err := instances[1].vm.LastAccepted(ctx)
			gomega.Ω(err).To(gomega.BeNil())
			gomega.Ω(lastAccepted).To(gomega.Equal(blk.ID()))

			results := blk.(*chain.StatelessBlock).Results()
			gomega.Ω(results).Should(gomega.HaveLen(1))
			gomega.Ω(results[0].Success).Should(gomega.BeTrue())
			gomega.Ω(results[0].Units).Should(gomega.Equal(uint64(transferTxFee)))
			gomega.Ω(results[0].Output).Should(gomega.BeNil())
		})

		ginkgo.By("ensure balance is updated", func() {
			balance, err := instances[1].cli.Balance(context.Background(), sender, ids.Empty)
			gomega.Ω(err).To(gomega.BeNil())
			gomega.Ω(balance).To(gomega.Equal(uint64(9899528)))
			balance2, err := instances[1].cli.Balance(context.Background(), sender2, ids.Empty)
			gomega.Ω(err).To(gomega.BeNil())
			gomega.Ω(balance2).To(gomega.Equal(uint64(100000)))
		})
	})

	ginkgo.It("ensure multiple txs work ", func() {
		ginkgo.By("transfer funds again", func() {
			submit, _, _, err := instances[1].cli.GenerateTransaction(
				context.Background(),
				&actions.Transfer{
					To:    rsender2,
					Value: 101,
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			accept := expectBlk(instances[1])
			results := accept()
			gomega.Ω(results).Should(gomega.HaveLen(1))
			gomega.Ω(results[0].Success).Should(gomega.BeTrue())

			balance2, err := instances[1].cli.Balance(context.Background(), sender2, ids.Empty)
			gomega.Ω(err).To(gomega.BeNil())
			gomega.Ω(balance2).To(gomega.Equal(uint64(100101)))
		})
	})

	ginkgo.It("Test processing block handling", func() {
		var accept, accept2 func() []*chain.Result

		ginkgo.By("create processing tip", func() {
			submit, _, _, err := instances[1].cli.GenerateTransaction(
				context.Background(),
				&actions.Transfer{
					To:    rsender2,
					Value: 200,
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			accept = expectBlk(instances[1])

			submit, _, _, err = instances[1].cli.GenerateTransaction(
				context.Background(),
				&actions.Transfer{
					To:    rsender2,
					Value: 201,
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			accept2 = expectBlk(instances[1])
		})

		ginkgo.By("clear processing tip", func() {
			results := accept()
			gomega.Ω(results).Should(gomega.HaveLen(1))
			gomega.Ω(results[0].Success).Should(gomega.BeTrue())
			results = accept2()
			gomega.Ω(results).Should(gomega.HaveLen(1))
			gomega.Ω(results[0].Success).Should(gomega.BeTrue())
		})
	})

	ginkgo.It("ensure mempool works", func() {
		ginkgo.By("fail Gossip TransferTx to a stale node when missing previous blocks", func() {
			submit, _, _, err := instances[1].cli.GenerateTransaction(
				context.Background(),
				&actions.Transfer{
					To:    rsender2,
					Value: 203,
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())

			err = instances[1].vm.Gossiper().TriggerGossip(context.TODO())
			gomega.Ω(err).Should(gomega.BeNil())

			// mempool in 0 should be 1 (old amount), since gossip/submit failed
			gomega.Ω(instances[0].vm.Mempool().Len(context.TODO())).Should(gomega.Equal(1))
		})
	})

	ginkgo.It("ensure unprocessed tip works", func() {
		ginkgo.By("import accepted blocks to instance 2", func() {
			ctx := context.TODO()
			o := instances[1]
			blks := []snowman.Block{}
			next, err := o.vm.LastAccepted(ctx)
			gomega.Ω(err).Should(gomega.BeNil())
			for {
				blk, err := o.vm.GetBlock(ctx, next)
				gomega.Ω(err).Should(gomega.BeNil())
				blks = append([]snowman.Block{blk}, blks...)
				if blk.Height() == 1 {
					break
				}
				next = blk.Parent()
			}

			n := instances[2]
			blk1, err := n.vm.ParseBlock(ctx, blks[0].Bytes())
			gomega.Ω(err).Should(gomega.BeNil())
			err = blk1.Verify(ctx)
			gomega.Ω(err).Should(gomega.BeNil())

			// Parse tip
			blk2, err := n.vm.ParseBlock(ctx, blks[1].Bytes())
			gomega.Ω(err).Should(gomega.BeNil())
			blk3, err := n.vm.ParseBlock(ctx, blks[2].Bytes())
			gomega.Ω(err).Should(gomega.BeNil())

			// Verify tip
			err = blk2.Verify(ctx)
			gomega.Ω(err).Should(gomega.BeNil())
			err = blk3.Verify(ctx)
			gomega.Ω(err).Should(gomega.BeNil())

			// Accept tip
			err = blk1.Accept(ctx)
			gomega.Ω(err).Should(gomega.BeNil())
			err = blk2.Accept(ctx)
			gomega.Ω(err).Should(gomega.BeNil())
			err = blk3.Accept(ctx)
			gomega.Ω(err).Should(gomega.BeNil())

			// Parse another
			blk4, err := n.vm.ParseBlock(ctx, blks[3].Bytes())
			gomega.Ω(err).Should(gomega.BeNil())
			err = blk4.Verify(ctx)
			gomega.Ω(err).Should(gomega.BeNil())
			err = blk4.Accept(ctx)
			gomega.Ω(err).Should(gomega.BeNil())
		})
	})

	ginkgo.It("processes valid index transactions (w/block listening)", func() {
		// Clear previous txs on instance 0
		accept := expectBlk(instances[0])
		accept() // don't care about results

		// Subscribe to blocks
		blocksPort, err := instances[0].cli.BlocksPort(context.TODO())
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(blocksPort).Should(gomega.Not(gomega.Equal(0)))
		tcpURI := fmt.Sprintf("127.0.0.1:%d", blocksPort)
		cli, err := vm.NewBlockRPCClient(tcpURI)
		gomega.Ω(err).Should(gomega.BeNil())

		// Fetch balances
		balance, err := instances[0].cli.Balance(context.TODO(), sender, ids.Empty)
		gomega.Ω(err).Should(gomega.BeNil())

		// Send tx
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		transfer := &actions.Transfer{
			To:    other.PublicKey(),
			Value: 1,
		}

		submit, rawTx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			transfer,
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())

		gomega.Ω(err).Should(gomega.BeNil())
		accept = expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		// Read item from connection
		blk, lresults, err := cli.Listen(instances[0].vm)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(len(blk.Txs)).Should(gomega.Equal(1))
		tx := blk.Txs[0].Action.(*actions.Transfer)
		gomega.Ω(tx.Asset).To(gomega.Equal(ids.Empty))
		gomega.Ω(tx.Value).To(gomega.Equal(uint64(1)))
		gomega.Ω(lresults).Should(gomega.Equal(results))
		gomega.Ω(cli.Close()).Should(gomega.BeNil())

		// Check balance modifications are correct
		balancea, err := instances[0].cli.Balance(context.TODO(), sender, ids.Empty)
		gomega.Ω(err).Should(gomega.BeNil())
		g, err := instances[0].cli.Genesis(context.TODO())
		gomega.Ω(err).Should(gomega.BeNil())
		r := g.Rules(instances[0].chainID, time.Now().Unix())
		gomega.Ω(balance).Should(gomega.Equal(balancea + rawTx.MaxUnits(r) + 1))
	})

	ginkgo.It("processes valid index transactions (w/streaming verification)", func() {
		// Create streaming client
		decisionsPort, err := instances[0].cli.DecisionsPort(context.TODO())
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(decisionsPort).Should(gomega.Not(gomega.Equal(0)))
		tcpURI := fmt.Sprintf("127.0.0.1:%d", decisionsPort)
		cli, err := vm.NewDecisionRPCClient(tcpURI)
		gomega.Ω(err).Should(gomega.BeNil())

		// Create tx
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		transfer := &actions.Transfer{
			To:    other.PublicKey(),
			Value: 1,
		}
		_, tx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			transfer,
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())

		// Submit tx and accept block
		gomega.Ω(cli.IssueTx(tx)).Should(gomega.BeNil())
		for instances[0].vm.Mempool().Len(context.TODO()) == 0 {
			// We need to wait for mempool to be populated because issuance will
			// return as soon as bytes are on the channel.
			hutils.Outf("{{yellow}}waiting for mempool to return non-zero txs{{/}}\n")
			time.Sleep(500 * time.Millisecond)
		}
		gomega.Ω(err).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		// Read decision from connection
		txID, dErr, result, err := cli.Listen()
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(txID).Should(gomega.Equal(tx.ID()))
		gomega.Ω(dErr).Should(gomega.BeNil())
		gomega.Ω(result.Success).Should(gomega.BeTrue())
		gomega.Ω(result).Should(gomega.Equal(results[0]))
		gomega.Ω(cli.Close()).Should(gomega.BeNil())
	})

	ginkgo.It("mints a new asset", func() {
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		aother := utils.Address(other.PublicKey())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Mint{
				To:    other.PublicKey(),
				Asset: asset1,
				Value: 10,
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		balance, err := instances[0].cli.Balance(context.TODO(), aother, asset1)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(10)))
		balance, err = instances[0].cli.Balance(context.TODO(), sender, asset1)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(0)))
		balance, err = instances[0].cli.Balance(context.TODO(), aother, ids.Empty)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(0)))
	})

	ginkgo.It("rejects empty mint", func() {
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Mint{
				To:    other.PublicKey(),
				Asset: ids.GenerateTestID(),
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background()).Error()).
			Should(gomega.ContainSubstring("Uint64 field is not populated"))
	})

	ginkgo.It("rejects duplicate mint", func() {
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		aother := utils.Address(other.PublicKey())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Mint{
				To:    other.PublicKey(),
				Asset: asset1,
				Value: 10,
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeFalse())
		gomega.Ω(string(result.Output)).
			Should(gomega.ContainSubstring("asset already exists"))

		balance, err := instances[0].cli.Balance(context.TODO(), aother, asset1)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(0)))
	})

	ginkgo.It("rejects mint of native token", func() {
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		aother := utils.Address(other.PublicKey())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Mint{
				To:    other.PublicKey(),
				Value: 10,
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeFalse())
		gomega.Ω(string(result.Output)).
			Should(gomega.ContainSubstring("cannot mint native asset"))

		balance, err := instances[0].cli.Balance(context.TODO(), aother, ids.Empty)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(0)))
	})

	ginkgo.It("mints another new asset (to self)", func() {
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Mint{
				To:    rsender,
				Asset: asset2,
				Value: 10,
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		balance, err := instances[0].cli.Balance(context.TODO(), sender, asset2)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(10)))
	})

	ginkgo.It("mints another new asset (to self) on another account", func() {
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Mint{
				To:    rsender2,
				Asset: asset3,
				Value: 10,
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		balance, err := instances[0].cli.Balance(context.TODO(), sender2, asset3)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(10)))
	})

	ginkgo.It("create simple order (want 3, give 2) untracked", func() {
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.CreateOrder{
				In:     asset3,
				Out:    asset2,
				Rate:   actions.CreateRate(2), // 1 asset3 = 2 asset2
				Supply: 5,                     // put half of balance
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		balance, err := instances[0].cli.Balance(context.TODO(), sender, asset2)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(5)))

		orders, err := instances[0].cli.Orders(context.TODO(), actions.PairID(asset3, asset2))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(0))
	})

	ginkgo.It("create simple order (want 2, give 3) tracked", func() {
		submit, tx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.CreateOrder{
				In:     asset2,
				Out:    asset3,
				Rate:   actions.CreateRate(0.25), // 1 asset3 = 4 asset2
				Supply: 5,                        // put half of balance
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		balance, err := instances[0].cli.Balance(context.TODO(), sender2, asset3)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(5)))

		orders, err := instances[0].cli.Orders(context.TODO(), actions.PairID(asset2, asset3))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		gomega.Ω(order.ID).Should(gomega.Equal(tx.ID()))
		gomega.Ω(order.Rate).Should(gomega.Equal(actions.CreateRate(0.25)))
		gomega.Ω(order.Owner).Should(gomega.Equal(rsender2))
		gomega.Ω(order.Remaining).Should(gomega.Equal(uint64(5)))
	})

	ginkgo.It("create order with insufficient balance", func() {
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.CreateOrder{
				In:     asset2,
				Out:    asset3,
				Rate:   actions.CreateRate(0.25), // 1 asset3 = 4 asset2
				Supply: 5,                        // put half of balance
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeFalse())
		gomega.Ω(string(result.Output)).
			Should(gomega.ContainSubstring("invalid balance"))
	})

	ginkgo.It("fill order with insufficient balance", func() {
		orders, err := instances[0].cli.Orders(context.TODO(), actions.PairID(asset2, asset3))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.FillOrder{
				Order: order.ID,
				Owner: order.Owner,
				In:    asset2,
				Out:   asset3,
				Value: 10, // rate of this order is 4 asset2 = 1 asset3
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeFalse())
		gomega.Ω(string(result.Output)).
			Should(gomega.ContainSubstring("invalid balance"))
	})

	ginkgo.It("fill order with insufficient output", func() {
		orders, err := instances[0].cli.Orders(context.TODO(), actions.PairID(asset2, asset3))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.FillOrder{
				Order: order.ID,
				Owner: order.Owner,
				In:    asset2,
				Out:   asset3,
				Value: 1, // rate of this order is 4 asset2 = 1 asset3
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeFalse())
		gomega.Ω(string(result.Output)).
			Should(gomega.ContainSubstring("insufficient output"))
	})

	ginkgo.It("fill order with sufficient balance", func() {
		orders, err := instances[0].cli.Orders(context.TODO(), actions.PairID(asset2, asset3))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.FillOrder{
				Order: order.ID,
				Owner: order.Owner,
				In:    asset2,
				Out:   asset3,
				Value: 4, // rate of this order is 4 asset2 = 1 asset3
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeTrue())
		or, err := actions.UnmarshalOrderResult(result.Output)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(or.In).Should(gomega.Equal(uint64(4)))
		gomega.Ω(or.Out).Should(gomega.Equal(uint64(1)))
		gomega.Ω(or.Remaining).Should(gomega.Equal(uint64(4)))

		balance, err := instances[0].cli.Balance(context.TODO(), sender, asset3)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(1)))
		balance, err = instances[0].cli.Balance(context.TODO(), sender, asset2)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(1)))

		orders, err = instances[0].cli.Orders(context.TODO(), actions.PairID(asset2, asset3))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order = orders[0]
		gomega.Ω(order.Remaining).Should(gomega.Equal(uint64(4)))
	})

	ginkgo.It("close order with wrong owner", func() {
		orders, err := instances[0].cli.Orders(context.TODO(), actions.PairID(asset2, asset3))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.CloseOrder{
				Order: order.ID,
				Out:   asset3,
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeFalse())
		gomega.Ω(string(result.Output)).
			Should(gomega.ContainSubstring("unauthorized"))
	})

	ginkgo.It("close order", func() {
		orders, err := instances[0].cli.Orders(context.TODO(), actions.PairID(asset2, asset3))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.CloseOrder{
				Order: order.ID,
				Out:   asset3,
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeTrue())

		balance, err := instances[0].cli.Balance(context.TODO(), sender2, asset3)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(9)))
		balance, err = instances[0].cli.Balance(context.TODO(), sender2, asset2)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(4)))

		orders, err = instances[0].cli.Orders(context.TODO(), actions.PairID(asset2, asset3))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(0))
	})

	ginkgo.It("create simple order (want 2, give 3) tracked from another account", func() {
		submit, tx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.CreateOrder{
				In:     asset2,
				Out:    asset3,
				Rate:   actions.CreateRate(1.5), // 1.5 asset2 = 1 asset3
				Supply: 1,                       // put half of balance
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		balance, err := instances[0].cli.Balance(context.TODO(), sender, asset3)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(0)))

		orders, err := instances[0].cli.Orders(context.TODO(), actions.PairID(asset2, asset3))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		gomega.Ω(order.ID).Should(gomega.Equal(tx.ID()))
		gomega.Ω(order.Rate).Should(gomega.Equal(actions.CreateRate(1.5)))
		gomega.Ω(order.Owner).Should(gomega.Equal(rsender))
		gomega.Ω(order.Remaining).Should(gomega.Equal(uint64(1)))
	})

	ginkgo.It("fill order with more than enough value", func() {
		orders, err := instances[0].cli.Orders(context.TODO(), actions.PairID(asset2, asset3))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.FillOrder{
				Order: order.ID,
				Owner: order.Owner,
				In:    asset2,
				Out:   asset3,
				Value: 4, // rate of this order is 1.5 asset2 = 1 asset3
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeTrue())
		or, err := actions.UnmarshalOrderResult(result.Output)
		gomega.Ω(err).Should(gomega.BeNil())
		// Calculations:
		// 4 * 1.5 = 6 asset3 expected
		// 6 - 1 = 5 over remaining
		// 5 / 1.5 (ignore divisor) = 3.333...
		gomega.Ω(or.In).Should(gomega.Equal(uint64(3)))
		gomega.Ω(or.Out).Should(gomega.Equal(uint64(1)))
		gomega.Ω(or.Remaining).Should(gomega.Equal(uint64(0)))
		panic("broken")

		balance, err := instances[0].cli.Balance(context.TODO(), sender, asset3)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(0)))
		balance, err = instances[0].cli.Balance(context.TODO(), sender, asset2)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(4)))

		orders, err = instances[0].cli.Orders(context.TODO(), actions.PairID(asset2, asset3))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(0))
	})
})

func expectBlk(i instance) func() []*chain.Result {
	ctx := context.TODO()

	// manually signal ready
	i.vm.Builder().TriggerBuild()
	// manually ack ready sig as in engine
	<-i.toEngine

	blk, err := i.vm.BuildBlock(ctx)
	gomega.Ω(err).To(gomega.BeNil())
	gomega.Ω(blk).To(gomega.Not(gomega.BeNil()))

	gomega.Ω(blk.Verify(ctx)).To(gomega.BeNil())
	gomega.Ω(blk.Status()).To(gomega.Equal(choices.Processing))

	err = i.vm.SetPreference(ctx, blk.ID())
	gomega.Ω(err).To(gomega.BeNil())

	return func() []*chain.Result {
		gomega.Ω(blk.Accept(ctx)).To(gomega.BeNil())
		gomega.Ω(blk.Status()).To(gomega.Equal(choices.Accepted))

		lastAccepted, err := i.vm.LastAccepted(ctx)
		gomega.Ω(err).To(gomega.BeNil())
		gomega.Ω(lastAccepted).To(gomega.Equal(blk.ID()))
		return blk.(*chain.StatelessBlock).Results()
	}
}

var _ common.AppSender = &appSender{}

type appSender struct {
	next      int
	instances []instance
}

func (app *appSender) SendAppGossip(ctx context.Context, appGossipBytes []byte) error {
	n := len(app.instances)
	sender := app.instances[app.next].nodeID
	app.next++
	app.next %= n
	return app.instances[app.next].vm.AppGossip(ctx, sender, appGossipBytes)
}

func (*appSender) SendAppRequest(context.Context, set.Set[ids.NodeID], uint32, []byte) error {
	return nil
}

func (*appSender) SendAppResponse(context.Context, ids.NodeID, uint32, []byte) error {
	return nil
}

func (*appSender) SendAppGossipSpecific(context.Context, set.Set[ids.NodeID], []byte) error {
	return nil
}

func (*appSender) SendCrossChainAppRequest(context.Context, ids.ID, uint32, []byte) error {
	return nil
}

func (*appSender) SendCrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
	return nil
}

// feeModifier adjusts the base fee to ensure we don't create a duplicate tx
type feeModifier struct{ unitPrice uint64 } //nolint:unused

func (f feeModifier) Base(base *chain.Base) { //nolint:unused
	base.UnitPrice = f.unitPrice
}

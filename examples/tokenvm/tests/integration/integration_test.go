// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

// integration implements the integration tests.
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
	"github.com/ava-labs/hypersdk/consts"
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

const (
	transferTxFee = 400 /* base fee */ + 40 /* transfer fee */
)

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
	ginkgo.RunSpecs(t, "indexvm integration test suites")
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
	factory *auth.DirectFactory
	rsender crypto.PublicKey
	sender  string

	priv2    crypto.PrivateKey
	factory2 *auth.DirectFactory
	rsender2 crypto.PublicKey
	sender2  string

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
	factory = auth.NewDirectFactory(priv)
	rsender = priv.PublicKey()
	sender = utils.Address(rsender)
	log.Debug(
		"generated key",
		zap.String("addr", sender),
		zap.String("pk", hex.EncodeToString(priv[:])),
	)

	priv2, err = crypto.GeneratePrivateKey()
	gomega.Ω(err).Should(gomega.BeNil())
	factory2 = auth.NewDirectFactory(priv2)
	rsender2 = priv2.PublicKey()
	sender2 = utils.Address(rsender2)
	log.Debug(
		"generated key",
		zap.String("addr", sender2),
		zap.String("pk", hex.EncodeToString(priv2[:])),
	)

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
			[]byte(`{"parallelism":3, "testMode":true, "logLevel":"debug"}`),
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
			u, l, exists, err := cli.Balance(context.Background(), alloc.Address)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(exists).Should(gomega.BeTrue())
			gomega.Ω(u).Should(gomega.Equal(alloc.Balance - g.StateLockup*2))
			gomega.Ω(l).Should(gomega.Equal(g.StateLockup * 2 /* perms + balance */))
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
			balance, _, _, err := instances[1].cli.Balance(context.Background(), sender)
			gomega.Ω(err).To(gomega.BeNil())
			gomega.Ω(balance).To(gomega.Equal(uint64(9897512)))
			balance2, _, _, err := instances[1].cli.Balance(context.Background(), sender2)
			gomega.Ω(err).To(gomega.BeNil())
			gomega.Ω(balance2).To(gomega.Equal(uint64(97952)))
		})
	})

	ginkgo.It("ensure multiple txs work ", func() {
		ginkgo.By("transfer funds again", func() {
			submit, _, _, err := instances[1].cli.GenerateTransaction(
				context.Background(),
				&actions.Transfer{
					To:    rsender2,
					Value: 101, // account already exists so can be less than state lockup
				},
				factory,
			)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
			accept := expectBlk(instances[1])
			results := accept()
			gomega.Ω(results).Should(gomega.HaveLen(1))
			gomega.Ω(results[0].Success).Should(gomega.BeTrue())

			balance2, _, _, err := instances[1].cli.Balance(context.Background(), sender2)
			gomega.Ω(err).To(gomega.BeNil())
			gomega.Ω(balance2).To(gomega.Equal(uint64(98053)))
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
		ubal, lbal, _, err := instances[0].cli.Balance(context.TODO(), sender)
		gomega.Ω(err).Should(gomega.BeNil())

		// Send tx
		schema := hutils.ToID([]byte("schema"))
		index := &actions.Index{
			// Don't set a parent
			Schema:  schema,
			Content: []byte{1, 0, 1, 0, 1, 0, 1},
			// Don't set a royalty
		}
		contentID := index.ContentID()

		submit, rawTx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			index,
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
		tx := blk.Txs[0].Action.(*actions.Index)
		gomega.Ω(tx.Schema).To(gomega.Equal(schema))
		gomega.Ω(tx.ContentID()).To(gomega.Equal(contentID))
		gomega.Ω(lresults).Should(gomega.Equal(results))
		gomega.Ω(cli.Close()).Should(gomega.BeNil())

		// Check balance modifications are correct
		ubala, lbala, _, err := instances[0].cli.Balance(context.TODO(), sender)
		gomega.Ω(err).Should(gomega.BeNil())
		g, err := instances[0].cli.Genesis(context.TODO())
		gomega.Ω(err).Should(gomega.BeNil())
		r := g.Rules(instances[0].chainID, time.Now().Unix())
		gomega.Ω(ubal).Should(gomega.Equal(ubala + rawTx.MaxUnits(r)))
		gomega.Ω(lbal).Should(gomega.Equal(lbala))
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
		index := &actions.Index{
			// Don't set a parent
			Schema:  ids.GenerateTestID(),
			Content: []byte{1, 0, 1, 0, 1, 0, 2},
		}
		_, tx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			index,
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

	ginkgo.It("rejects invalid schema", func() {
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Index{
				// Don't set a parent
				Schema:  ids.Empty,
				Content: []byte{1, 0, 1, 0, 1, 0, 1},
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background()).Error()).
			Should(gomega.ContainSubstring("ID field is not populated"))
	})

	ginkgo.It("rejects invalid content", func() {
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Index{
				// Don't set a parent
				Schema:  hutils.ToID([]byte("schema")),
				Content: make([]byte, 64_000),
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background()).Error()).
			Should(gomega.ContainSubstring("size is larger than limit"))
	})

	ginkgo.It("rejects empty balance", func() {
		tpriv, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		tfactory := auth.NewDirectFactory(tpriv)

		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Index{
				Parent:  ids.GenerateTestID(),
				Schema:  ids.GenerateTestID(),
				Content: []byte{1, 0, 1, 0, 1, 0, 1},
			},
			tfactory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background()).Error()).
			Should(gomega.ContainSubstring("not allowed"))
	})

	ginkgo.It("processes multiple index transactions", func() {
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Index{
				Parent:  ids.GenerateTestID(),
				Schema:  hutils.ToID([]byte("schema")),
				Content: []byte{1, 0, 1, 0, 1, 0, 1, 0},
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())

		submit, _, _, err = instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Index{
				// Don't set a parent
				Schema:  hutils.ToID([]byte("schema")),
				Content: []byte{1, 0, 1, 0, 1, 0, 1, 1},
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(2))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())
		gomega.Ω(results[1].Success).Should(gomega.BeTrue())
	})

	var claimedContent ids.ID
	ginkgo.It("claim content", func() {
		// Fetch balances
		ubal, lbal, _, err := instances[0].cli.Balance(context.TODO(), sender)
		gomega.Ω(err).Should(gomega.BeNil())

		// Process transaction
		index := &actions.Index{
			Schema:  hutils.ToID([]byte("schema")),
			Content: []byte{1, 0, 1, 0, 1, 0, 1, 0},
			Royalty: 10,
		}
		claimedContent = index.ContentID()
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			index,
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeTrue())

		// Check balances
		ubala, lbala, _, err := instances[0].cli.Balance(context.TODO(), sender)
		gomega.Ω(err).Should(gomega.BeNil())
		g, err := instances[0].cli.Genesis(context.TODO())
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(ubal).Should(gomega.Equal(ubala + result.Units + g.StateLockup))
		gomega.Ω(lbala).Should(gomega.Equal(lbal + g.StateLockup))
	})

	ginkgo.It("reference content", func() {
		// Fetch balances
		ubal, lbal, _, err := instances[0].cli.Balance(context.TODO(), sender)
		gomega.Ω(err).Should(gomega.BeNil())
		ubal2, lbal2, _, err := instances[0].cli.Balance(context.TODO(), sender2)
		gomega.Ω(err).Should(gomega.BeNil())

		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Index{
				Parent:   claimedContent,
				Schema:   hutils.ToID([]byte("schema2")),
				Content:  []byte{1, 1, 1},
				Searcher: rsender,
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

		// Check balances
		ubala, lbala, _, err := instances[0].cli.Balance(context.TODO(), sender)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(ubal).Should(gomega.Equal(ubala - 10)) // royalty paid automatically
		gomega.Ω(lbal).Should(gomega.Equal(lbala))
		ubala2, lbala2, _, err := instances[0].cli.Balance(context.TODO(), sender2)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(ubal2).Should(gomega.Equal(ubala2 + result.Units + 10))
		gomega.Ω(lbala2).Should(gomega.Equal(lbal2))
	})

	var cid1 ids.ID
	ginkgo.It("reference more content", func() {
		rowner, _, err := instances[0].cli.Content(context.TODO(), claimedContent)
		gomega.Ω(err).Should(gomega.BeNil())
		owner, err := utils.ParseAddress(rowner)
		gomega.Ω(err).Should(gomega.BeNil())

		// Process transaction
		act := &actions.Index{
			Parent:   claimedContent,
			Schema:   hutils.ToID([]byte("schema2")),
			Content:  []byte{1, 1, 1},
			Searcher: owner,
			Royalty:  1,
		}
		cid1 = act.ContentID()
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			act,
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeTrue())
	})

	ginkgo.It("reference missing parent owner", func() {
		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Index{
				Parent:  claimedContent,
				Schema:  hutils.ToID([]byte("schema2")),
				Content: []byte{1, 1, 1},
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeFalse())
		gomega.Ω(string(result.Output)).
			Should(gomega.ContainSubstring("key not specified"))
		// missing key for owner
	})

	ginkgo.It("reference unclaimed content (w/owner)", func() {
		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Index{
				Parent:   ids.ID{1},
				Schema:   hutils.ToID([]byte("schema2")),
				Content:  []byte{1, 1, 1},
				Searcher: rsender,
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeFalse())
		gomega.Ω(result.Output).Should(gomega.Equal(actions.OutputWrongOwner))
	})

	ginkgo.It("modify content", func() {
		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Modify{
				Content: claimedContent,
				Royalty: 20,
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
	})

	ginkgo.It("modify content (same royalty)", func() {
		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Modify{
				Content: claimedContent,
				Royalty: 20,
			},
			factory,
			feeModifier{unitPrice: 10}, // make sure we don't create a duplicate account
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeFalse())
		gomega.Ω(result.Output).Should(gomega.Equal(actions.OutputInvalidObject))
	})

	ginkgo.It("modify content (wrong owner)", func() {
		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Modify{
				Content: claimedContent,
				Royalty: 25,
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeFalse())
		gomega.Ω(result.Output).Should(gomega.Equal(actions.OutputWrongOwner))
	})

	ginkgo.It("reference content after change (w/schema change)", func() {
		// Fetch balances
		ubal, _, _, err := instances[0].cli.Balance(context.TODO(), sender2)
		gomega.Ω(err).Should(gomega.BeNil())

		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Index{
				Parent:   claimedContent,
				Schema:   hutils.ToID([]byte("schema3")),
				Content:  []byte{1, 1, 1},
				Searcher: rsender,
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

		// Confirm paid updated royalty
		ubala, _, _, err := instances[0].cli.Balance(context.TODO(), sender2)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(ubal).Should(gomega.Equal(ubala + result.Units + 20))
	})

	ginkgo.It("duplicate claim", func() {
		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Index{
				Schema:  hutils.ToID([]byte("schema")),
				Content: []byte{1, 0, 1, 0, 1, 0, 1, 0},
				Royalty: 10,
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeFalse())
		gomega.Ω(string(result.Output)).Should(gomega.ContainSubstring("content already exists"))
	})

	ginkgo.It("unindex not as owner", func() {
		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Unindex{
				Content: claimedContent,
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeFalse())
		gomega.Ω(string(result.Output)).Should(gomega.ContainSubstring("wrong owner"))
	})

	ginkgo.It("unindex unclaimed", func() {
		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Unindex{
				Content: ids.ID{1},
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeFalse())
		gomega.Ω(string(result.Output)).Should(gomega.Equal("content does not exist"))
	})

	ginkgo.It("unindex claim", func() {
		// Fetch balances
		ubal, _, _, err := instances[0].cli.Balance(context.TODO(), sender)
		gomega.Ω(err).Should(gomega.BeNil())

		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Unindex{
				Content: claimedContent,
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

		// Check balances
		ubala, lbala, _, err := instances[0].cli.Balance(context.TODO(), sender)
		gomega.Ω(err).Should(gomega.BeNil())
		g, err := instances[0].cli.Genesis(context.TODO())
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(ubal).Should(gomega.Equal(ubala + result.Units - g.StateLockup))
		gomega.Ω(lbala).Should(gomega.Equal(g.StateLockup * 2))
	})

	ginkgo.It("send balance back to main account to test invalid balance", func() {
		// Send balance back to main account
		u, _, _, err := instances[0].cli.Balance(context.TODO(), sender2)
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Transfer{
				To:    rsender,
				Value: u - transferTxFee,
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

		// Process transaction
		submit, _, _, err = instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Index{
				Schema:  hutils.ToID([]byte("schema")),
				Content: []byte{1, 0, 1, 0, 1, 0, 1, 0},
				Royalty: 15,
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background()).Error()).
			Should(gomega.ContainSubstring("invalid balance"))
	})

	var cid2 ids.ID
	ginkgo.It("reclaim content (w/different account)", func() {
		// Transfer balance back to account 2
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Transfer{
				To:    rsender2,
				Value: 10_000,
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

		// Fetch balances
		ubal, lbal, _, err := instances[0].cli.Balance(context.TODO(), sender2)
		gomega.Ω(err).Should(gomega.BeNil())

		// Process transaction
		index := &actions.Index{
			Schema:  hutils.ToID([]byte("schema")),
			Content: []byte{1, 0, 1, 0, 1, 0, 1, 0},
			Royalty: 15,
		}
		cid2 = index.ContentID()
		gomega.Ω(index.ContentID()).Should(gomega.Equal(claimedContent))
		submit, _, _, err = instances[0].cli.GenerateTransaction(
			context.Background(),
			index,
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept = expectBlk(instances[0])
		results = accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result = results[0]
		gomega.Ω(result.Success).Should(gomega.BeTrue())

		// Check balances
		ubala, lbala, _, err := instances[0].cli.Balance(context.TODO(), sender2)
		gomega.Ω(err).Should(gomega.BeNil())
		g, err := instances[0].cli.Genesis(context.TODO())
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(ubal).Should(gomega.Equal(ubala + result.Units + g.StateLockup))
		gomega.Ω(lbala).Should(gomega.Equal(lbal + g.StateLockup))
	})

	ginkgo.It("fail to claim content by modifying", func() {
		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Modify{
				Content: ids.ID{10},
				Royalty: 10,
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
		gomega.Ω(result.Output).Should(gomega.Equal(actions.OutputContentMissing))

		// Check index lookup
		searcher, royalty, err := instances[0].cli.Content(context.TODO(), ids.ID{10})
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(searcher).Should(gomega.Equal(""))
		gomega.Ω(royalty).Should(gomega.Equal(uint64(0)))
	})

	var lastClaim ids.ID
	ginkgo.It("claim content to cause clear failure", func() {
		// Process transaction
		index := &actions.Index{
			Schema:  hutils.ToID([]byte("schema")),
			Content: []byte{1, 0, 1, 0, 1, 0, 1, 0, 1},
			Royalty: 10,
		}
		lastClaim = index.ContentID()
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			index,
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeTrue())
	})

	ginkgo.It("attempt to clear non-empty account", func() {
		// Fetch balances
		ubal, lbal, _, err := instances[0].cli.Balance(context.TODO(), sender)
		gomega.Ω(err).Should(gomega.BeNil())

		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Clear{
				To: rsender2,
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
		gomega.Ω(result.Output).Should(gomega.Equal(actions.OutputAccountNotEmpty))

		// Check balances
		ubala, lbala, _, err := instances[0].cli.Balance(context.TODO(), sender)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(ubal).Should(gomega.Equal(ubala + result.Units))
		gomega.Ω(lbala).Should(gomega.Equal(lbal))
	})

	ginkgo.It("clear root account", func() {
		// Unindex last content
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Unindex{
				Content: lastClaim,
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

		// Process transaction
		submit, _, _, err = instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Clear{
				To: rsender2,
			},
			factory,
			feeModifier{unitPrice: 2}, // need to make different than failed
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept = expectBlk(instances[0])
		results = accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result = results[0]
		gomega.Ω(result.Success).Should(gomega.BeTrue())

		// Check balances
		ubal, lbal, exists, err := instances[0].cli.Balance(context.TODO(), sender)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(ubal).Should(gomega.Equal(uint64(0)))
		gomega.Ω(lbal).Should(gomega.Equal(uint64(0)))
		gomega.Ω(exists).Should(gomega.BeFalse())
	})

	ginkgo.It("insufficient send to root", func() {
		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Transfer{
				To:    rsender,
				Value: 100,
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeFalse())
		gomega.Ω(string(result.Output)).
			Should(gomega.ContainSubstring("could not subtract unlocked"))
	})

	ginkgo.It("sufficient send to root", func() {
		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Transfer{
				To:    rsender,
				Value: 10_000,
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

		// Check balances
		ubal, lbal, exists, err := instances[0].cli.Balance(context.TODO(), sender)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(ubal).Should(gomega.Equal(uint64(7952)))
		gomega.Ω(lbal).Should(gomega.Equal(uint64(2048)))
		gomega.Ω(exists).Should(gomega.BeTrue())
	})

	ginkgo.It("clear and authorize root", func() {
		// Process transaction
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Clear{
				To: rsender2,
			},
			factory,
			feeModifier{unitPrice: 11}, // make sure we don't create a duplicate account
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeTrue())

		// Add root as authorized signer
		submit, _, _, err = instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Authorize{
				Actor:             rsender2,
				Signer:            rsender,
				ActionPermissions: consts.MaxUint8,
				MiscPermissions:   consts.MaxUint8,
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept = expectBlk(instances[0])
		results = accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result = results[0]
		gomega.Ω(result.Success).Should(gomega.BeTrue())

		// Revoke [sender2] creds
		dfactory := auth.NewDelegateFactory(rsender2, priv, true)
		submit, _, _, err = instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Authorize{
				Actor:  rsender2,
				Signer: rsender2,
			},
			dfactory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept = expectBlk(instances[0])
		results = accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result = results[0]
		gomega.Ω(result.Success).Should(gomega.BeTrue())

		// Attempt to transfer direct as [sender2]
		submit, _, _, err = instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Transfer{
				To:    rsender,
				Value: 10_001,
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background()).Error()).
			Should(gomega.ContainSubstring("not allowed"))

		// Transfer direct as [sender] on behalf of [sender2], with no money in
		// [sender]
		tpriv, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err = instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Transfer{
				To:    tpriv.PublicKey(),
				Value: 10_000,
			},
			dfactory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept = expectBlk(instances[0])
		results = accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result = results[0]
		gomega.Ω(result.Success).Should(gomega.BeTrue())

		// Clear all items owned by [sender2]
		submit, _, _, err = instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Unindex{
				Content: cid1,
			},
			dfactory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept = expectBlk(instances[0])
		results = accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result = results[0]
		gomega.Ω(result.Success).Should(gomega.BeTrue())

		submit, _, _, err = instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Unindex{
				Content: cid2,
			},
			dfactory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept = expectBlk(instances[0])
		results = accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result = results[0]
		gomega.Ω(result.Success).Should(gomega.BeTrue())

		// Clear [sender2] using [sender]
		submit, _, _, err = instances[0].cli.GenerateTransaction(
			context.Background(),
			&actions.Clear{
				To: tpriv.PublicKey(),
			},
			dfactory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept = expectBlk(instances[0])
		results = accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result = results[0]
		gomega.Ω(result.Success).Should(gomega.BeTrue())
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
type feeModifier struct{ unitPrice uint64 }

func (f feeModifier) Base(base *chain.Base) {
	base.UnitPrice = f.unitPrice
}

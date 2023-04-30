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
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	avago_version "github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/fatih/color"
	ginkgo "github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/rpc"
	hutils "github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/vm"

	"github.com/ava-labs/hypersdk/examples/tokenvm/actions"
	"github.com/ava-labs/hypersdk/examples/tokenvm/auth"
	tconsts "github.com/ava-labs/hypersdk/examples/tokenvm/consts"
	"github.com/ava-labs/hypersdk/examples/tokenvm/controller"
	"github.com/ava-labs/hypersdk/examples/tokenvm/genesis"
	trpc "github.com/ava-labs/hypersdk/examples/tokenvm/rpc"
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

	asset1   []byte
	asset1ID ids.ID
	asset2   []byte
	asset2ID ids.ID
	asset3   []byte
	asset3ID ids.ID

	// when used with embedded VMs
	genesisBytes []byte
	instances    []instance

	gen *genesis.Genesis
)

type instance struct {
	chainID            ids.ID
	nodeID             ids.NodeID
	vm                 *vm.VM
	toEngine           chan common.Message
	JSONRPCServer      *httptest.Server
	TokenJSONRPCServer *httptest.Server
	WebSocketServer    *httptest.Server
	cli                *rpc.JSONRPCClient // clients for embedded VMs
	tcli               *trpc.JSONRPCClient
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

	asset1 = []byte("1")
	asset2 = []byte("2")
	asset3 = []byte("3")

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
		sk, err := bls.NewSecretKey()
		gomega.Ω(err).Should(gomega.BeNil())
		l, err := logFactory.Make(nodeID.String())
		gomega.Ω(err).Should(gomega.BeNil())
		dname, err := os.MkdirTemp("", fmt.Sprintf("%s-chainData", nodeID.String()))
		gomega.Ω(err).Should(gomega.BeNil())
		snowCtx := &snow.Context{
			NetworkID:      networkID,
			SubnetID:       subnetID,
			ChainID:        chainID,
			NodeID:         nodeID,
			Log:            l,
			ChainDataDir:   dname,
			Metrics:        metrics.NewOptionalGatherer(),
			PublicKey:      bls.PublicFromSecretKey(sk),
			WarpSigner:     warp.NewSigner(sk, chainID),
			ValidatorState: &validators.TestState{},
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
				`{"parallelism":3, "testMode":true, "logLevel":"debug", "trackedPairs":["*"]}`,
			),
			toEngine,
			nil,
			app,
		)
		gomega.Ω(err).Should(gomega.BeNil())

		var hd map[string]*common.HTTPHandler
		hd, err = v.CreateHandlers(context.TODO())
		gomega.Ω(err).Should(gomega.BeNil())

		jsonRPCServer := httptest.NewServer(hd[rpc.JSONRPCEndpoint].Handler)
		tjsonRPCServer := httptest.NewServer(hd[trpc.JSONRPCEndpoint].Handler)
		webSocketServer := httptest.NewServer(hd[rpc.WebSocketEndpoint].Handler)
		instances[i] = instance{
			chainID:            snowCtx.ChainID,
			nodeID:             snowCtx.NodeID,
			vm:                 v,
			toEngine:           toEngine,
			JSONRPCServer:      jsonRPCServer,
			TokenJSONRPCServer: tjsonRPCServer,
			WebSocketServer:    webSocketServer,
			cli:                rpc.NewJSONRPCClient(jsonRPCServer.URL),
			tcli:               trpc.NewJSONRPCClient(tjsonRPCServer.URL, snowCtx.ChainID),
		}

		// Force sync ready (to mimic bootstrapping from genesis)
		v.ForceReady()
	}

	// Verify genesis allocations loaded correctly (do here otherwise test may
	// check during and it will be inaccurate)
	for _, inst := range instances {
		cli := inst.tcli
		g, err := cli.Genesis(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())

		csupply := uint64(0)
		for _, alloc := range g.CustomAllocation {
			balance, err := cli.Balance(context.Background(), alloc.Address, ids.Empty)
			gomega.Ω(err).Should(gomega.BeNil())
			gomega.Ω(balance).Should(gomega.Equal(alloc.Balance))
			csupply += alloc.Balance
		}
		exists, metadata, supply, owner, warp, err := cli.Asset(context.Background(), ids.Empty)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(exists).Should(gomega.BeTrue())
		gomega.Ω(string(metadata)).Should(gomega.Equal(tconsts.Symbol))
		gomega.Ω(supply).Should(gomega.Equal(csupply))
		gomega.Ω(owner).Should(gomega.Equal(utils.Address(crypto.EmptyPublicKey)))
		gomega.Ω(warp).Should(gomega.BeFalse())
	}

	app.instances = instances
	color.Blue("created %d VMs", vms)
})

var _ = ginkgo.AfterSuite(func() {
	for _, iv := range instances {
		iv.JSONRPCServer.Close()
		iv.TokenJSONRPCServer.Close()
		iv.WebSocketServer.Close()
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
			_, _, _, err := cli.Accepted(context.Background())
			gomega.Ω(err).Should(gomega.BeNil())
		}
	})

	var transferTxRoot *chain.Transaction
	ginkgo.It("Gossip TransferTx to a different node", func() {
		ginkgo.By("issue TransferTx", func() {
			parser, err := instances[0].tcli.Parser(context.Background())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, transferTx, _, err := instances[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				nil,
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
			actionRegistry, authRegistry := instances[0].vm.Registry()
			tx := chain.NewTx(
				&chain.Base{
					ChainID:   instances[0].chainID,
					Timestamp: 0,
					UnitPrice: 1000,
				},
				nil,
				&actions.Transfer{
					To:    rsender2,
					Value: 110,
				},
			)
			// Must do manual construction to avoid `tx.Sign` error (would fail with
			// 0 timestamp)
			msg, err := tx.Digest(actionRegistry)
			gomega.Ω(err).To(gomega.BeNil())
			auth, err := factory.Sign(msg, tx.Action)
			gomega.Ω(err).To(gomega.BeNil())
			tx.Auth = auth
			p := codec.NewWriter(consts.MaxInt)
			gomega.Ω(tx.Marshal(p, actionRegistry, authRegistry)).To(gomega.BeNil())
			gomega.Ω(p.Err()).To(gomega.BeNil())
			_, err = instances[0].cli.SubmitTx(
				context.Background(),
				p.Bytes(),
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
			balance, err := instances[1].tcli.Balance(context.Background(), sender, ids.Empty)
			gomega.Ω(err).To(gomega.BeNil())
			gomega.Ω(balance).To(gomega.Equal(uint64(9899528)))
			balance2, err := instances[1].tcli.Balance(context.Background(), sender2, ids.Empty)
			gomega.Ω(err).To(gomega.BeNil())
			gomega.Ω(balance2).To(gomega.Equal(uint64(100000)))
		})
	})

	ginkgo.It("ensure multiple txs work ", func() {
		ginkgo.By("transfer funds again", func() {
			parser, err := instances[1].tcli.Parser(context.Background())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, _, _, err := instances[1].cli.GenerateTransaction(
				context.Background(),
				parser,
				nil,
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

			balance2, err := instances[1].tcli.Balance(context.Background(), sender2, ids.Empty)
			gomega.Ω(err).To(gomega.BeNil())
			gomega.Ω(balance2).To(gomega.Equal(uint64(100101)))
		})
	})

	ginkgo.It("Test processing block handling", func() {
		var accept, accept2 func() []*chain.Result

		ginkgo.By("create processing tip", func() {
			parser, err := instances[1].tcli.Parser(context.Background())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, _, _, err := instances[1].cli.GenerateTransaction(
				context.Background(),
				parser,
				nil,
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
				parser,
				nil,
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
			parser, err := instances[1].tcli.Parser(context.Background())
			gomega.Ω(err).Should(gomega.BeNil())
			submit, _, _, err := instances[1].cli.GenerateTransaction(
				context.Background(),
				parser,
				nil,
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
		cli, err := rpc.NewWebSocketClient(instances[0].WebSocketServer.URL)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(cli.RegisterBlocks()).Should(gomega.BeNil())

		// Fetch balances
		balance, err := instances[0].tcli.Balance(context.TODO(), sender, ids.Empty)
		gomega.Ω(err).Should(gomega.BeNil())

		// Send tx
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		transfer := &actions.Transfer{
			To:    other.PublicKey(),
			Value: 1,
		}

		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, rawTx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
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
		blk, lresults, err := cli.ListenBlock(context.TODO(), parser)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(len(blk.Txs)).Should(gomega.Equal(1))
		tx := blk.Txs[0].Action.(*actions.Transfer)
		gomega.Ω(tx.Asset).To(gomega.Equal(ids.Empty))
		gomega.Ω(tx.Value).To(gomega.Equal(uint64(1)))
		gomega.Ω(lresults).Should(gomega.Equal(results))

		// Check balance modifications are correct
		balancea, err := instances[0].tcli.Balance(context.TODO(), sender, ids.Empty)
		gomega.Ω(err).Should(gomega.BeNil())
		g, err := instances[0].tcli.Genesis(context.TODO())
		gomega.Ω(err).Should(gomega.BeNil())
		r := g.Rules(time.Now().Unix())
		maxUnits, err := rawTx.MaxUnits(r)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(balancea + maxUnits + 1))

		// Close connection when done
		gomega.Ω(cli.Close()).Should(gomega.BeNil())
	})

	ginkgo.It("processes valid index transactions (w/streaming verification)", func() {
		// Create streaming client
		cli, err := rpc.NewWebSocketClient(instances[0].WebSocketServer.URL)
		gomega.Ω(err).Should(gomega.BeNil())

		// Create tx
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		transfer := &actions.Transfer{
			To:    other.PublicKey(),
			Value: 1,
		}
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		_, tx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			transfer,
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())

		// Submit tx and accept block
		gomega.Ω(cli.RegisterTx(tx)).Should(gomega.BeNil())
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
		txID, dErr, result, err := cli.ListenTx(context.TODO())
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(txID).Should(gomega.Equal(tx.ID()))
		gomega.Ω(dErr).Should(gomega.BeNil())
		gomega.Ω(result.Success).Should(gomega.BeTrue())
		gomega.Ω(result).Should(gomega.Equal(results[0]))

		// Close connection when done
		gomega.Ω(cli.Close()).Should(gomega.BeNil())
	})

	ginkgo.It("mint an asset that doesn't exist", func() {
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		assetID := ids.GenerateTestID()
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.MintAsset{
				To:    other.PublicKey(),
				Asset: assetID,
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
			Should(gomega.ContainSubstring("asset missing"))

		exists, _, _, _, _, err := instances[0].tcli.Asset(context.TODO(), assetID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(exists).Should(gomega.BeFalse())
	})

	ginkgo.It("create a new asset (no metadata)", func() {
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, tx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.CreateAsset{
				Metadata: nil,
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		assetID := tx.ID()
		balance, err := instances[0].tcli.Balance(context.TODO(), sender, assetID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(0)))
		exists, metadata, supply, owner, warp, err := instances[0].tcli.Asset(
			context.TODO(),
			assetID,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(exists).Should(gomega.BeTrue())
		gomega.Ω(metadata).Should(gomega.HaveLen(0))
		gomega.Ω(supply).Should(gomega.Equal(uint64(0)))
		gomega.Ω(owner).Should(gomega.Equal(sender))
		gomega.Ω(warp).Should(gomega.BeFalse())
	})

	ginkgo.It("create asset with too long of metadata", func() {
		actionRegistry, authRegistry := instances[0].vm.Registry()
		tx := chain.NewTx(
			&chain.Base{
				ChainID:   instances[0].chainID,
				Timestamp: time.Now().Unix(),
				UnitPrice: 1000,
			},
			nil,
			&actions.CreateAsset{
				Metadata: make([]byte, actions.MaxMetadataSize*2),
			},
		)
		// Must do manual construction to avoid `tx.Sign` error (would fail with
		// too large)
		msg, err := tx.Digest(actionRegistry)
		gomega.Ω(err).To(gomega.BeNil())
		auth, err := factory.Sign(msg, tx.Action)
		gomega.Ω(err).To(gomega.BeNil())
		tx.Auth = auth
		p := codec.NewWriter(consts.MaxInt)
		gomega.Ω(tx.Marshal(p, actionRegistry, authRegistry)).To(gomega.BeNil())
		gomega.Ω(p.Err()).To(gomega.BeNil())
		_, err = instances[0].cli.SubmitTx(
			context.Background(),
			p.Bytes(),
		)
		gomega.Ω(err.Error()).Should(gomega.ContainSubstring("size is larger than limit"))
	})

	ginkgo.It("create a new asset (simple metadata)", func() {
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, tx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.CreateAsset{
				Metadata: asset1,
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		asset1ID = tx.ID()
		balance, err := instances[0].tcli.Balance(context.TODO(), sender, asset1ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(0)))

		exists, metadata, supply, owner, warp, err := instances[0].tcli.Asset(
			context.TODO(),
			asset1ID,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(exists).Should(gomega.BeTrue())
		gomega.Ω(metadata).Should(gomega.Equal(asset1))
		gomega.Ω(supply).Should(gomega.Equal(uint64(0)))
		gomega.Ω(owner).Should(gomega.Equal(sender))
		gomega.Ω(warp).Should(gomega.BeFalse())
	})

	ginkgo.It("mint a new asset", func() {
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.MintAsset{
				To:    rsender2,
				Asset: asset1ID,
				Value: 15,
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		balance, err := instances[0].tcli.Balance(context.TODO(), sender2, asset1ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(15)))
		balance, err = instances[0].tcli.Balance(context.TODO(), sender, asset1ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(0)))

		exists, metadata, supply, owner, warp, err := instances[0].tcli.Asset(
			context.TODO(),
			asset1ID,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(exists).Should(gomega.BeTrue())
		gomega.Ω(metadata).Should(gomega.Equal(asset1))
		gomega.Ω(supply).Should(gomega.Equal(uint64(15)))
		gomega.Ω(owner).Should(gomega.Equal(sender))
		gomega.Ω(warp).Should(gomega.BeFalse())
	})

	ginkgo.It("mint asset from wrong owner", func() {
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.MintAsset{
				To:    other.PublicKey(),
				Asset: asset1ID,
				Value: 10,
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
			Should(gomega.ContainSubstring("wrong owner"))

		exists, metadata, supply, owner, warp, err := instances[0].tcli.Asset(
			context.TODO(),
			asset1ID,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(exists).Should(gomega.BeTrue())
		gomega.Ω(metadata).Should(gomega.Equal(asset1))
		gomega.Ω(supply).Should(gomega.Equal(uint64(15)))
		gomega.Ω(owner).Should(gomega.Equal(sender))
		gomega.Ω(warp).Should(gomega.BeFalse())
	})

	ginkgo.It("burn new asset", func() {
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.BurnAsset{
				Asset: asset1ID,
				Value: 5,
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		balance, err := instances[0].tcli.Balance(context.TODO(), sender2, asset1ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(10)))
		balance, err = instances[0].tcli.Balance(context.TODO(), sender, asset1ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(0)))

		exists, metadata, supply, owner, warp, err := instances[0].tcli.Asset(
			context.TODO(),
			asset1ID,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(exists).Should(gomega.BeTrue())
		gomega.Ω(metadata).Should(gomega.Equal(asset1))
		gomega.Ω(supply).Should(gomega.Equal(uint64(10)))
		gomega.Ω(owner).Should(gomega.Equal(sender))
		gomega.Ω(warp).Should(gomega.BeFalse())
	})

	ginkgo.It("burn missing asset", func() {
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.BurnAsset{
				Asset: asset1ID,
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
			Should(gomega.ContainSubstring("invalid balance"))

		exists, metadata, supply, owner, warp, err := instances[0].tcli.Asset(
			context.TODO(),
			asset1ID,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(exists).Should(gomega.BeTrue())
		gomega.Ω(metadata).Should(gomega.Equal(asset1))
		gomega.Ω(supply).Should(gomega.Equal(uint64(10)))
		gomega.Ω(owner).Should(gomega.Equal(sender))
		gomega.Ω(warp).Should(gomega.BeFalse())
	})

	ginkgo.It("rejects empty mint", func() {
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		actionRegistry, authRegistry := instances[0].vm.Registry()
		tx := chain.NewTx(
			&chain.Base{
				ChainID:   instances[0].chainID,
				Timestamp: time.Now().Unix(),
				UnitPrice: 1000,
			},
			nil,
			&actions.MintAsset{
				To:    other.PublicKey(),
				Asset: asset1ID,
			},
		)
		// Must do manual construction to avoid `tx.Sign` error (would fail with
		// bad codec)
		msg, err := tx.Digest(actionRegistry)
		gomega.Ω(err).To(gomega.BeNil())
		auth, err := factory.Sign(msg, tx.Action)
		gomega.Ω(err).To(gomega.BeNil())
		tx.Auth = auth
		p := codec.NewWriter(consts.MaxInt)
		gomega.Ω(tx.Marshal(p, actionRegistry, authRegistry)).To(gomega.BeNil())
		gomega.Ω(p.Err()).To(gomega.BeNil())
		_, err = instances[0].cli.SubmitTx(
			context.Background(),
			p.Bytes(),
		)
		gomega.Ω(err.Error()).Should(gomega.ContainSubstring("Uint64 field is not populated"))
	})

	ginkgo.It("reject max mint", func() {
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.MintAsset{
				To:    rsender2,
				Asset: asset1ID,
				Value: consts.MaxUint64,
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
			Should(gomega.ContainSubstring("overflow"))

		balance, err := instances[0].tcli.Balance(context.TODO(), sender2, asset1ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(10)))
		balance, err = instances[0].tcli.Balance(context.TODO(), sender, asset1ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(0)))

		exists, metadata, supply, owner, warp, err := instances[0].tcli.Asset(
			context.TODO(),
			asset1ID,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(exists).Should(gomega.BeTrue())
		gomega.Ω(metadata).Should(gomega.Equal(asset1))
		gomega.Ω(supply).Should(gomega.Equal(uint64(10)))
		gomega.Ω(owner).Should(gomega.Equal(sender))
		gomega.Ω(warp).Should(gomega.BeFalse())
	})

	ginkgo.It("modify an existing asset", func() {
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.ModifyAsset{
				Asset:    asset1ID,
				Metadata: []byte("blah"),
				Owner:    crypto.EmptyPublicKey,
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		balance, err := instances[0].tcli.Balance(context.TODO(), sender2, asset1ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(10)))
		balance, err = instances[0].tcli.Balance(context.TODO(), sender, asset1ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(0)))

		exists, metadata, supply, owner, warp, err := instances[0].tcli.Asset(
			context.TODO(),
			asset1ID,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(exists).Should(gomega.BeTrue())
		gomega.Ω(metadata).Should(gomega.Equal([]byte("blah")))
		gomega.Ω(supply).Should(gomega.Equal(uint64(10)))
		gomega.Ω(owner).Should(gomega.Equal(utils.Address(crypto.EmptyPublicKey)))
		gomega.Ω(warp).Should(gomega.BeFalse())
	})

	ginkgo.It("modify an asset that doesn't exist", func() {
		assetID := ids.GenerateTestID()
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.ModifyAsset{
				Asset:    assetID,
				Metadata: []byte("cool"),
				Owner:    rsender,
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
			Should(gomega.ContainSubstring("asset missing"))

		exists, _, _, _, _, err := instances[0].tcli.Asset(context.TODO(), assetID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(exists).Should(gomega.BeFalse())
	})

	ginkgo.It("rejects mint of native token", func() {
		other, err := crypto.GeneratePrivateKey()
		gomega.Ω(err).Should(gomega.BeNil())
		actionRegistry, authRegistry := instances[0].vm.Registry()
		tx := chain.NewTx(
			&chain.Base{
				ChainID:   instances[0].chainID,
				Timestamp: time.Now().Unix(),
				UnitPrice: 1000,
			},
			nil,
			&actions.MintAsset{
				To:    other.PublicKey(),
				Value: 10,
			},
		)
		// Must do manual construction to avoid `tx.Sign` error (would fail with
		// bad codec)
		msg, err := tx.Digest(actionRegistry)
		gomega.Ω(err).To(gomega.BeNil())
		auth, err := factory.Sign(msg, tx.Action)
		gomega.Ω(err).To(gomega.BeNil())
		tx.Auth = auth
		p := codec.NewWriter(consts.MaxInt)
		gomega.Ω(tx.Marshal(p, actionRegistry, authRegistry)).To(gomega.BeNil())
		gomega.Ω(p.Err()).To(gomega.BeNil())
		_, err = instances[0].cli.SubmitTx(
			context.Background(),
			p.Bytes(),
		)
		gomega.Ω(err.Error()).Should(gomega.ContainSubstring("ID field is not populated"))
	})

	ginkgo.It("mints another new asset (to self)", func() {
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, tx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.CreateAsset{
				Metadata: asset2,
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())
		asset2ID = tx.ID()

		submit, _, _, err = instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.MintAsset{
				To:    rsender,
				Asset: asset2ID,
				Value: 10,
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept = expectBlk(instances[0])
		results = accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		balance, err := instances[0].tcli.Balance(context.TODO(), sender, asset2ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(10)))
	})

	ginkgo.It("mints another new asset (to self) on another account", func() {
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, tx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.CreateAsset{
				Metadata: asset3,
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())
		asset3ID = tx.ID()

		submit, _, _, err = instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.MintAsset{
				To:    rsender2,
				Asset: asset3ID,
				Value: 10,
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept = expectBlk(instances[0])
		results = accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		balance, err := instances[0].tcli.Balance(context.TODO(), sender2, asset3ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(10)))
	})

	ginkgo.It("create simple order (want 3, give 2)", func() {
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, tx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.CreateOrder{
				In:      asset3ID,
				InTick:  1,
				Out:     asset2ID,
				OutTick: 2,
				Supply:  4,
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		balance, err := instances[0].tcli.Balance(context.TODO(), sender, asset2ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(6)))

		orders, err := instances[0].tcli.Orders(context.TODO(), actions.PairID(asset3ID, asset2ID))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		gomega.Ω(order.ID).Should(gomega.Equal(tx.ID()))
		gomega.Ω(order.InTick).Should(gomega.Equal(uint64(1)))
		gomega.Ω(order.OutTick).Should(gomega.Equal(uint64(2)))
		gomega.Ω(order.Owner).Should(gomega.Equal(sender))
		gomega.Ω(order.Remaining).Should(gomega.Equal(uint64(4)))
	})

	ginkgo.It("create simple order with misaligned supply", func() {
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.CreateOrder{
				In:      asset2ID,
				InTick:  4,
				Out:     asset3ID,
				OutTick: 2,
				Supply:  5, // put half of balance
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
			Should(gomega.ContainSubstring("supply is misaligned"))
	})

	ginkgo.It("create simple order (want 2, give 3) tracked", func() {
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, tx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.CreateOrder{
				In:      asset2ID,
				InTick:  4,
				Out:     asset3ID,
				OutTick: 1,
				Supply:  5, // put half of balance
			},
			factory2,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		balance, err := instances[0].tcli.Balance(context.TODO(), sender2, asset3ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(5)))

		orders, err := instances[0].tcli.Orders(context.TODO(), actions.PairID(asset2ID, asset3ID))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		gomega.Ω(order.ID).Should(gomega.Equal(tx.ID()))
		gomega.Ω(order.InTick).Should(gomega.Equal(uint64(4)))
		gomega.Ω(order.OutTick).Should(gomega.Equal(uint64(1)))
		gomega.Ω(order.Owner).Should(gomega.Equal(sender2))
		gomega.Ω(order.Remaining).Should(gomega.Equal(uint64(5)))
	})

	ginkgo.It("create order with insufficient balance", func() {
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.CreateOrder{
				In:      asset2ID,
				InTick:  5,
				Out:     asset3ID,
				OutTick: 1,
				Supply:  5, // put half of balance
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

	ginkgo.It("fill order with misaligned value", func() {
		orders, err := instances[0].tcli.Orders(context.TODO(), actions.PairID(asset2ID, asset3ID))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		owner, err := utils.ParseAddress(order.Owner)
		gomega.Ω(err).Should(gomega.BeNil())
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.FillOrder{
				Order: order.ID,
				Owner: owner,
				In:    asset2ID,
				Out:   asset3ID,
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
			Should(gomega.ContainSubstring("value is misaligned"))
	})

	ginkgo.It("fill order with insufficient balance", func() {
		orders, err := instances[0].tcli.Orders(context.TODO(), actions.PairID(asset2ID, asset3ID))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		owner, err := utils.ParseAddress(order.Owner)
		gomega.Ω(err).Should(gomega.BeNil())
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.FillOrder{
				Order: order.ID,
				Owner: owner,
				In:    asset2ID,
				Out:   asset3ID,
				Value: 20, // rate of this order is 4 asset2 = 1 asset3
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

	ginkgo.It("fill order with sufficient balance", func() {
		orders, err := instances[0].tcli.Orders(context.TODO(), actions.PairID(asset2ID, asset3ID))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		owner, err := utils.ParseAddress(order.Owner)
		gomega.Ω(err).Should(gomega.BeNil())
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.FillOrder{
				Order: order.ID,
				Owner: owner,
				In:    asset2ID,
				Out:   asset3ID,
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

		balance, err := instances[0].tcli.Balance(context.TODO(), sender, asset3ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(1)))
		balance, err = instances[0].tcli.Balance(context.TODO(), sender, asset2ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(2)))

		orders, err = instances[0].tcli.Orders(context.TODO(), actions.PairID(asset2ID, asset3ID))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order = orders[0]
		gomega.Ω(order.Remaining).Should(gomega.Equal(uint64(4)))
	})

	ginkgo.It("close order with wrong owner", func() {
		orders, err := instances[0].tcli.Orders(context.TODO(), actions.PairID(asset2ID, asset3ID))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.CloseOrder{
				Order: order.ID,
				Out:   asset3ID,
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
		orders, err := instances[0].tcli.Orders(context.TODO(), actions.PairID(asset2ID, asset3ID))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.CloseOrder{
				Order: order.ID,
				Out:   asset3ID,
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

		balance, err := instances[0].tcli.Balance(context.TODO(), sender, asset3ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(1)))
		balance, err = instances[0].tcli.Balance(context.TODO(), sender, asset2ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(2)))
		balance, err = instances[0].tcli.Balance(context.TODO(), sender2, asset3ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(9)))
		balance, err = instances[0].tcli.Balance(context.TODO(), sender2, asset2ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(4)))

		orders, err = instances[0].tcli.Orders(context.TODO(), actions.PairID(asset2ID, asset3ID))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(0))
	})

	ginkgo.It("create simple order (want 2, give 3) tracked from another account", func() {
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, tx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.CreateOrder{
				In:      asset2ID,
				InTick:  2,
				Out:     asset3ID,
				OutTick: 1,
				Supply:  1,
			},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())
		accept := expectBlk(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		gomega.Ω(results[0].Success).Should(gomega.BeTrue())

		balance, err := instances[0].tcli.Balance(context.TODO(), sender, asset3ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(0)))

		orders, err := instances[0].tcli.Orders(context.TODO(), actions.PairID(asset2ID, asset3ID))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		gomega.Ω(order.ID).Should(gomega.Equal(tx.ID()))
		gomega.Ω(order.InTick).Should(gomega.Equal(uint64(2)))
		gomega.Ω(order.OutTick).Should(gomega.Equal(uint64(1)))
		gomega.Ω(order.Owner).Should(gomega.Equal(sender))
		gomega.Ω(order.Remaining).Should(gomega.Equal(uint64(1)))
	})

	ginkgo.It("fill order with more than enough value", func() {
		orders, err := instances[0].tcli.Orders(context.TODO(), actions.PairID(asset2ID, asset3ID))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(1))
		order := orders[0]
		owner, err := utils.ParseAddress(order.Owner)
		gomega.Ω(err).Should(gomega.BeNil())
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.FillOrder{
				Order: order.ID,
				Owner: owner,
				In:    asset2ID,
				Out:   asset3ID,
				Value: 4,
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
		gomega.Ω(or.In).Should(gomega.Equal(uint64(2)))
		gomega.Ω(or.Out).Should(gomega.Equal(uint64(1)))
		gomega.Ω(or.Remaining).Should(gomega.Equal(uint64(0)))

		balance, err := instances[0].tcli.Balance(context.TODO(), sender, asset3ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(0)))
		balance, err = instances[0].tcli.Balance(context.TODO(), sender, asset2ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(4)))
		balance, err = instances[0].tcli.Balance(context.TODO(), sender2, asset3ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(10)))
		balance, err = instances[0].tcli.Balance(context.TODO(), sender2, asset2ID)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(balance).Should(gomega.Equal(uint64(2)))

		orders, err = instances[0].tcli.Orders(context.TODO(), actions.PairID(asset2ID, asset3ID))
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(orders).Should(gomega.HaveLen(0))
	})

	ginkgo.It("import warp message with nil when expected", func() {
		actionRegistry, authRegistry := instances[0].vm.Registry()
		tx := chain.NewTx(
			&chain.Base{
				ChainID:   instances[0].chainID,
				Timestamp: time.Now().Unix(),
				UnitPrice: 1000,
			},
			nil,
			&actions.ImportAsset{},
		)
		// Must do manual construction to avoid `tx.Sign` error (would fail with
		// empty warp)
		msg, err := tx.Digest(actionRegistry)
		gomega.Ω(err).To(gomega.BeNil())
		auth, err := factory.Sign(msg, tx.Action)
		gomega.Ω(err).To(gomega.BeNil())
		tx.Auth = auth
		p := codec.NewWriter(consts.MaxInt)
		gomega.Ω(tx.Marshal(p, actionRegistry, authRegistry)).To(gomega.BeNil())
		gomega.Ω(p.Err()).To(gomega.BeNil())
		_, err = instances[0].cli.SubmitTx(
			context.Background(),
			p.Bytes(),
		)
		gomega.Ω(err.Error()).Should(gomega.ContainSubstring("expected warp message"))
	})

	ginkgo.It("import warp message empty", func() {
		wm, err := warp.NewMessage(&warp.UnsignedMessage{}, &warp.BitSetSignature{})
		gomega.Ω(err).Should(gomega.BeNil())
		actionRegistry, authRegistry := instances[0].vm.Registry()
		tx := chain.NewTx(
			&chain.Base{
				ChainID:   instances[0].chainID,
				Timestamp: time.Now().Unix(),
				UnitPrice: 1000,
			},
			wm,
			&actions.ImportAsset{},
		)
		// Must do manual construction to avoid `tx.Sign` error (would fail with
		// empty warp)
		msg, err := tx.Digest(actionRegistry)
		gomega.Ω(err).To(gomega.BeNil())
		auth, err := factory.Sign(msg, tx.Action)
		gomega.Ω(err).To(gomega.BeNil())
		tx.Auth = auth
		p := codec.NewWriter(consts.MaxInt)
		gomega.Ω(tx.Marshal(p, actionRegistry, authRegistry)).To(gomega.BeNil())
		gomega.Ω(p.Err()).To(gomega.BeNil())
		_, err = instances[0].cli.SubmitTx(
			context.Background(),
			p.Bytes(),
		)
		gomega.Ω(err.Error()).Should(gomega.ContainSubstring("empty warp payload"))
	})

	ginkgo.It("import with wrong payload", func() {
		uwm, err := warp.NewUnsignedMessage(ids.Empty, ids.Empty, []byte("hello"))
		gomega.Ω(err).Should(gomega.BeNil())
		wm, err := warp.NewMessage(uwm, &warp.BitSetSignature{})
		gomega.Ω(err).Should(gomega.BeNil())
		actionRegistry, authRegistry := instances[0].vm.Registry()
		tx := chain.NewTx(
			&chain.Base{
				ChainID:   instances[0].chainID,
				Timestamp: time.Now().Unix(),
				UnitPrice: 1000,
			},
			wm,
			&actions.ImportAsset{},
		)
		// Must do manual construction to avoid `tx.Sign` error (would fail with
		// invalid object)
		msg, err := tx.Digest(actionRegistry)
		gomega.Ω(err).To(gomega.BeNil())
		auth, err := factory.Sign(msg, tx.Action)
		gomega.Ω(err).To(gomega.BeNil())
		tx.Auth = auth
		p := codec.NewWriter(consts.MaxInt)
		gomega.Ω(tx.Marshal(p, actionRegistry, authRegistry)).To(gomega.BeNil())
		gomega.Ω(p.Err()).To(gomega.BeNil())
		_, err = instances[0].cli.SubmitTx(
			context.Background(),
			p.Bytes(),
		)
		gomega.Ω(err.Error()).Should(gomega.ContainSubstring("insufficient length for input"))
	})

	ginkgo.It("import with invalid payload", func() {
		wt := &actions.WarpTransfer{}
		wtb, err := wt.Marshal()
		gomega.Ω(err).Should(gomega.BeNil())
		uwm, err := warp.NewUnsignedMessage(ids.Empty, ids.Empty, wtb)
		gomega.Ω(err).Should(gomega.BeNil())
		wm, err := warp.NewMessage(uwm, &warp.BitSetSignature{})
		gomega.Ω(err).Should(gomega.BeNil())
		actionRegistry, authRegistry := instances[0].vm.Registry()
		tx := chain.NewTx(
			&chain.Base{
				ChainID:   instances[0].chainID,
				Timestamp: time.Now().Unix(),
				UnitPrice: 1000,
			},
			wm,
			&actions.ImportAsset{},
		)
		// Must do manual construction to avoid `tx.Sign` error (would fail with
		// invalid object)
		msg, err := tx.Digest(actionRegistry)
		gomega.Ω(err).To(gomega.BeNil())
		auth, err := factory.Sign(msg, tx.Action)
		gomega.Ω(err).To(gomega.BeNil())
		tx.Auth = auth
		p := codec.NewWriter(consts.MaxInt)
		gomega.Ω(tx.Marshal(p, actionRegistry, authRegistry)).To(gomega.BeNil())
		gomega.Ω(p.Err()).To(gomega.BeNil())
		_, err = instances[0].cli.SubmitTx(
			context.Background(),
			p.Bytes(),
		)
		gomega.Ω(err.Error()).Should(gomega.ContainSubstring("Uint64 field is not populated"))
	})

	ginkgo.It("import with wrong destination", func() {
		wt := &actions.WarpTransfer{
			To:     rsender,
			Asset:  ids.GenerateTestID(),
			Value:  100,
			Return: false,
			Reward: 100,
			TxID:   ids.GenerateTestID(),
		}
		wtb, err := wt.Marshal()
		gomega.Ω(err).Should(gomega.BeNil())
		uwm, err := warp.NewUnsignedMessage(ids.Empty, ids.Empty, wtb)
		gomega.Ω(err).Should(gomega.BeNil())
		wm, err := warp.NewMessage(uwm, &warp.BitSetSignature{})
		gomega.Ω(err).Should(gomega.BeNil())
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			wm,
			&actions.ImportAsset{},
			factory,
		)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(submit(context.Background())).Should(gomega.BeNil())

		// Build block with no context (should fail)
		instances[0].vm.Builder().TriggerBuild()
		<-instances[0].toEngine
		blk, err := instances[0].vm.BuildBlock(context.TODO())
		gomega.Ω(err).To(gomega.Not(gomega.BeNil()))
		gomega.Ω(blk).To(gomega.BeNil())

		// Build block with context
		accept := expectBlkWithContext(instances[0])
		results := accept()
		gomega.Ω(results).Should(gomega.HaveLen(1))
		result := results[0]
		gomega.Ω(result.Success).Should(gomega.BeFalse())
		gomega.Ω(string(result.Output)).Should(gomega.ContainSubstring("warp verification failed"))
	})

	ginkgo.It("export native asset", func() {
		dest := ids.GenerateTestID()
		loan, err := instances[0].tcli.Loan(context.TODO(), ids.Empty, dest)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(loan).Should(gomega.Equal(uint64(0)))

		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, tx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.ExportAsset{
				To:          rsender,
				Asset:       ids.Empty,
				Value:       100,
				Return:      false,
				Reward:      10,
				Destination: dest,
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
		wt := &actions.WarpTransfer{
			To:     rsender,
			Asset:  ids.Empty,
			Value:  100,
			Return: false,
			Reward: 10,
			TxID:   tx.ID(),
		}
		wtb, err := wt.Marshal()
		gomega.Ω(err).Should(gomega.BeNil())
		wm, err := warp.NewUnsignedMessage(instances[0].chainID, dest, wtb)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(result.WarpMessage).Should(gomega.Equal(wm))

		loan, err = instances[0].tcli.Loan(context.TODO(), ids.Empty, dest)
		gomega.Ω(err).Should(gomega.BeNil())
		gomega.Ω(loan).Should(gomega.Equal(uint64(110)))
	})

	ginkgo.It("export native asset (invalid return)", func() {
		parser, err := instances[0].tcli.Parser(context.Background())
		gomega.Ω(err).Should(gomega.BeNil())
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			nil,
			&actions.ExportAsset{
				To:          rsender,
				Asset:       ids.Empty,
				Value:       100,
				Return:      true,
				Reward:      10,
				Destination: ids.GenerateTestID(),
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
		gomega.Ω(string(result.Output)).Should(gomega.ContainSubstring("not warp asset"))
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

// TODO: unify with expectBlk
func expectBlkWithContext(i instance) func() []*chain.Result {
	ctx := context.TODO()

	// manually signal ready
	i.vm.Builder().TriggerBuild()
	// manually ack ready sig as in engine
	<-i.toEngine

	bctx := &block.Context{PChainHeight: 1}
	blk, err := i.vm.BuildBlockWithContext(ctx, bctx)
	gomega.Ω(err).To(gomega.BeNil())
	gomega.Ω(blk).To(gomega.Not(gomega.BeNil()))
	cblk := blk.(block.WithVerifyContext)

	gomega.Ω(cblk.VerifyWithContext(ctx, bctx)).To(gomega.BeNil())
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

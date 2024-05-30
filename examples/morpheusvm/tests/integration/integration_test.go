// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/fatih/color"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/crypto/secp256r1"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/actions"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/auth"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/controller"
	"github.com/ava-labs/hypersdk/examples/morpheusvm/genesis"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/vm"

	hbls "github.com/ava-labs/hypersdk/crypto/bls"
	lconsts "github.com/ava-labs/hypersdk/examples/morpheusvm/consts"
	lrpc "github.com/ava-labs/hypersdk/examples/morpheusvm/rpc"
	hutils "github.com/ava-labs/hypersdk/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
)

var (
	logFactory logging.Factory
	log        logging.Logger

	requestTimeout time.Duration
	vms            int

	priv    ed25519.PrivateKey
	pk      ed25519.PublicKey
	factory *auth.ED25519Factory
	addr    codec.Address
	addrStr string

	priv2    ed25519.PrivateKey
	pk2      ed25519.PublicKey
	factory2 *auth.ED25519Factory
	addr2    codec.Address
	addrStr2 string

	priv3    ed25519.PrivateKey
	pk3      ed25519.PublicKey
	factory3 *auth.ED25519Factory
	addr3    codec.Address
	addrStr3 string

	// when used with embedded VMs
	genesisBytes []byte
	instances    []instance
	blocks       []snowman.Block

	networkID uint32
	gen       *genesis.Genesis
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
	ginkgo.RunSpecs(t, "morpheusvm integration test suites")
}

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
}

type instance struct {
	chainID           ids.ID
	nodeID            ids.NodeID
	vm                *vm.VM
	toEngine          chan common.Message
	JSONRPCServer     *httptest.Server
	BaseJSONRPCServer *httptest.Server
	WebSocketServer   *httptest.Server
	cli               *rpc.JSONRPCClient // clients for embedded VMs
	lcli              *lrpc.JSONRPCClient
}

var _ = ginkgo.BeforeSuite(func() {
	require := require.New(ginkgo.GinkgoT())

	log.Info("VMID", zap.Stringer("id", lconsts.ID))
	require.Greater(vms, 1)

	var err error
	priv, err = ed25519.GeneratePrivateKey()
	require.NoError(err)
	pk = priv.PublicKey()
	factory = auth.NewED25519Factory(priv)
	addr = auth.NewED25519Address(pk)
	addrStr = codec.MustAddressBech32(lconsts.HRP, addr)
	log.Debug(
		"generated key",
		zap.String("addr", addrStr),
		zap.String("pk", hex.EncodeToString(priv[:])),
	)

	priv2, err = ed25519.GeneratePrivateKey()
	require.NoError(err)
	pk2 = priv2.PublicKey()
	factory2 = auth.NewED25519Factory(priv2)
	addr2 = auth.NewED25519Address(pk2)
	addrStr2 = codec.MustAddressBech32(lconsts.HRP, addr2)
	log.Debug(
		"generated key",
		zap.String("addr", addrStr2),
		zap.String("pk", hex.EncodeToString(priv2[:])),
	)

	priv3, err = ed25519.GeneratePrivateKey()
	require.NoError(err)
	pk3 = priv3.PublicKey()
	factory3 = auth.NewED25519Factory(priv3)
	addr3 = auth.NewED25519Address(pk3)
	addrStr3 = codec.MustAddressBech32(lconsts.HRP, addr3)
	log.Debug(
		"generated key",
		zap.String("addr", addrStr3),
		zap.String("pk", hex.EncodeToString(priv3[:])),
	)

	// create embedded VMs
	instances = make([]instance, vms)

	gen = genesis.Default()
	gen.MinUnitPrice = fees.Dimensions{1, 1, 1, 1, 1}
	gen.MinBlockGap = 0
	gen.CustomAllocation = []*genesis.CustomAllocation{
		{
			Address: addrStr,
			Balance: 10_000_000,
		},
	}
	genesisBytes, err = json.Marshal(gen)
	require.NoError(err)

	networkID = uint32(1)
	subnetID := ids.GenerateTestID()
	chainID := ids.GenerateTestID()

	app := &appSender{}
	for i := range instances {
		nodeID := ids.GenerateTestNodeID()
		sk, err := bls.NewSecretKey()
		require.NoError(err)
		l, err := logFactory.Make(nodeID.String())
		require.NoError(err)
		dname, err := os.MkdirTemp("", fmt.Sprintf("%s-chainData", nodeID.String()))
		require.NoError(err)
		snowCtx := &snow.Context{
			NetworkID:      networkID,
			SubnetID:       subnetID,
			ChainID:        chainID,
			NodeID:         nodeID,
			Log:            l,
			ChainDataDir:   dname,
			Metrics:        metrics.NewOptionalGatherer(),
			PublicKey:      bls.PublicFromSecretKey(sk),
			ValidatorState: &validators.TestState{},
		}

		toEngine := make(chan common.Message, 1)
		db := memdb.New()

		v := controller.New()
		err = v.Initialize(
			context.TODO(),
			snowCtx,
			db,
			genesisBytes,
			nil,
			[]byte(
				`{"parallelism":3, "testMode":true, "logLevel":"debug"}`,
			),
			toEngine,
			nil,
			app,
		)
		require.NoError(err)

		var hd map[string]http.Handler
		hd, err = v.CreateHandlers(context.TODO())
		require.NoError(err)

		jsonRPCServer := httptest.NewServer(hd[rpc.JSONRPCEndpoint])
		ljsonRPCServer := httptest.NewServer(hd[lrpc.JSONRPCEndpoint])
		webSocketServer := httptest.NewServer(hd[rpc.WebSocketEndpoint])
		instances[i] = instance{
			chainID:           snowCtx.ChainID,
			nodeID:            snowCtx.NodeID,
			vm:                v,
			toEngine:          toEngine,
			JSONRPCServer:     jsonRPCServer,
			BaseJSONRPCServer: ljsonRPCServer,
			WebSocketServer:   webSocketServer,
			cli:               rpc.NewJSONRPCClient(jsonRPCServer.URL),
			lcli:              lrpc.NewJSONRPCClient(ljsonRPCServer.URL, snowCtx.NetworkID, snowCtx.ChainID),
		}

		// Force sync ready (to mimic bootstrapping from genesis)
		v.ForceReady()
	}

	// Verify genesis allocates loaded correctly (do here otherwise test may
	// check during and it will be inaccurate)
	for _, inst := range instances {
		cli := inst.lcli
		g, err := cli.Genesis(context.Background())
		require.NoError(err)

		csupply := uint64(0)
		for _, alloc := range g.CustomAllocation {
			balance, err := cli.Balance(context.Background(), alloc.Address)
			require.NoError(err)
			require.Equal(alloc.Balance, balance)
			log.Warn("balances", zap.String("addr", alloc.Address), zap.Uint64("bal", balance))
			csupply += alloc.Balance
		}
	}
	blocks = []snowman.Block{}

	app.instances = instances
	color.Blue("created %d VMs", vms)
})

var _ = ginkgo.AfterSuite(func() {
	require := require.New(ginkgo.GinkgoT())

	for _, iv := range instances {
		iv.JSONRPCServer.Close()
		iv.BaseJSONRPCServer.Close()
		iv.WebSocketServer.Close()
		err := iv.vm.Shutdown(context.TODO())
		require.NoError(err)
	}
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
			networkID, subnetID, chainID, err := cli.Network(context.Background())
			require.NoError(err)
			require.Equal(networkID, uint32(1))
			require.NotEqual(subnetID, ids.Empty)
			require.NotEqual(chainID, ids.Empty)
		}
	})
})

var _ = ginkgo.Describe("[Tx Processing]", func() {
	require := require.New(ginkgo.GinkgoT())

	// Unit explanation
	//
	// bandwidth: tx size
	// compute: 5 for signature, 1 for base, 1 for transfer
	// read: 2 keys reads
	// allocate: 1 key created with 1 chunk
	// write: 2 keys modified
	transferTxUnits := fees.Dimensions{188, 7, 14, 50, 26}
	transferTxFee := uint64(285)

	ginkgo.It("get currently accepted block ID", func() {
		for _, inst := range instances {
			cli := inst.cli
			_, _, _, err := cli.Accepted(context.Background())
			require.NoError(err)
		}
	})

	var transferTxRoot *chain.Transaction
	ginkgo.It("Gossip TransferTx to a different node", func() {
		ginkgo.By("check balance", func() {
			balance, err := instances[0].lcli.Balance(context.Background(), addrStr)
			require.NoError(err)
			require.Equal(balance, uint64(10_000_000))
		})

		ginkgo.By("issue TransferTx", func() {
			parser, err := instances[0].lcli.Parser(context.Background())
			require.NoError(err)
			submit, transferTx, _, err := instances[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{&actions.Transfer{
					To:    addr2,
					Value: 100_000, // must be more than StateLockup
				}},
				factory,
			)
			transferTxRoot = transferTx
			require.NoError(err)
			require.NoError(submit(context.Background()))
			require.Equal(instances[0].vm.Mempool().Len(context.Background()), 1)
		})

		ginkgo.By("skip duplicate", func() {
			_, err := instances[0].cli.SubmitTx(
				context.Background(),
				transferTxRoot.Bytes(),
			)
			require.Error(err)
		})

		ginkgo.By("send gossip from node 0 to 1", func() {
			err := instances[0].vm.Gossiper().Force(context.TODO())
			require.NoError(err)
		})

		ginkgo.By("skip invalid time", func() {
			tx := chain.NewTx(
				&chain.Base{
					ChainID:   instances[0].chainID,
					Timestamp: 0,
					MaxFee:    1000,
				},
				[]chain.Action{&actions.Transfer{
					To:    addr2,
					Value: 110,
				}},
			)
			// Must do manual construction to avoid `tx.Sign` error (would fail with
			// 0 timestamp)
			msg, err := tx.Digest()
			require.NoError(err)
			auth, err := factory.Sign(msg)
			require.NoError(err)
			tx.Auth = auth
			p := codec.NewWriter(0, consts.MaxInt) // test codec growth
			require.NoError(tx.Marshal(p))
			require.NoError(p.Err())
			_, err = instances[0].cli.SubmitTx(
				context.Background(),
				p.Bytes(),
			)
			require.Error(err)
		})

		ginkgo.By("skip duplicate (after gossip, which shouldn't clear)", func() {
			_, err := instances[0].cli.SubmitTx(
				context.Background(),
				transferTxRoot.Bytes(),
			)
			require.Error(err)
		})

		ginkgo.By("receive gossip in the node 1, and signal block build", func() {
			require.NoError(instances[1].vm.Builder().Force(context.TODO()))
			<-instances[1].toEngine
		})

		ginkgo.By("build block in the node 1", func() {
			ctx := context.TODO()
			blk, err := instances[1].vm.BuildBlock(ctx)
			require.NoError(err)

			require.NoError(blk.Verify(ctx))
			require.Equal(blk.Status(), choices.Processing)

			err = instances[1].vm.SetPreference(ctx, blk.ID())
			require.NoError(err)

			require.NoError(blk.Accept(ctx))
			require.Equal(blk.Status(), choices.Accepted)
			blocks = append(blocks, blk)

			lastAccepted, err := instances[1].vm.LastAccepted(ctx)
			require.NoError(err)
			require.Equal(lastAccepted, blk.ID())

			results := blk.(*chain.StatelessBlock).Results()
			require.Len(results, 1)
			require.True(results[0].Success)
			require.Len(results[0].Outputs[0], 0)
			require.Equal(results[0].Units, transferTxUnits)
			require.Equal(results[0].Fee, transferTxFee)
		})

		ginkgo.By("ensure balance is updated", func() {
			balance, err := instances[1].lcli.Balance(context.Background(), addrStr)
			require.NoError(err)
			require.Equal(balance, uint64(9_899_715))
			balance2, err := instances[1].lcli.Balance(context.Background(), addrStr2)
			require.NoError(err)
			require.Equal(balance2, uint64(100_000))
		})
	})

	ginkgo.It("ensure multiple txs work ", func() {
		ginkgo.By("transfer funds again", func() {
			parser, err := instances[1].lcli.Parser(context.Background())
			require.NoError(err)
			submit, _, _, err := instances[1].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{&actions.Transfer{
					To:    addr2,
					Value: 101,
				}},
				factory,
			)
			require.NoError(err)
			require.NoError(submit(context.Background()))
			accept := expectBlk(instances[1])
			results := accept(true)
			require.Len(results, 1)
			require.True(results[0].Success)
			require.Equal(results[0].Units, transferTxUnits)
			require.Equal(results[0].Fee, transferTxFee)

			balance2, err := instances[1].lcli.Balance(context.Background(), addrStr2)
			require.NoError(err)
			require.Equal(balance2, uint64(100_101))
		})

		ginkgo.By("transfer funds again (test storage keys)", func() {
			parser, err := instances[1].lcli.Parser(context.Background())
			require.NoError(err)

			submit, _, _, err := instances[1].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{&actions.Transfer{
					To:    addr2,
					Value: 102,
				}},
				factory,
			)
			require.NoError(err)
			require.NoError(submit(context.Background()))
			submit, _, _, err = instances[1].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{&actions.Transfer{
					To:    addr2,
					Value: 103,
				}},
				factory,
			)
			require.NoError(err)
			require.NoError(submit(context.Background()))
			submit, _, _, err = instances[1].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{&actions.Transfer{
					To:    addr3,
					Value: 104,
				}},
				factory,
			)
			require.NoError(err)
			require.NoError(submit(context.Background()))
			submit, _, _, err = instances[1].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{&actions.Transfer{
					To:    addr3,
					Value: 105,
				}},
				factory,
			)
			require.NoError(err)
			require.NoError(submit(context.Background()))

			// Ensure we can handle case where accepted block is not processed
			latestBlock := blocks[len(blocks)-1]
			latestBlock.(*chain.StatelessBlock).MarkUnprocessed()

			// Accept new block (should use accepted state)
			accept := expectBlk(instances[1])
			results := accept(true)

			// Check results
			require.Len(results, 4)
			for i := 0; i < 4; i++ {
				require.True(results[i].Success)
				require.Equal(results[i].Units, transferTxUnits)
				require.Equal(results[i].Fee, transferTxFee)
			}

			// Check end balance
			balance2, err := instances[1].lcli.Balance(context.Background(), addrStr2)
			require.NoError(err)
			require.Equal(balance2, uint64(100_306))
			balance3, err := instances[1].lcli.Balance(context.Background(), addrStr3)
			require.NoError(err)
			require.Equal(balance3, uint64(209))
		})
	})

	ginkgo.It("Test processing block handling", func() {
		var accept, accept2 func(bool) []*chain.Result

		ginkgo.By("create processing tip", func() {
			parser, err := instances[1].lcli.Parser(context.Background())
			require.NoError(err)
			submit, _, _, err := instances[1].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{&actions.Transfer{
					To:    addr2,
					Value: 200,
				}},
				factory,
			)
			require.NoError(err)
			require.NoError(submit(context.Background()))
			accept = expectBlk(instances[1])

			submit, _, _, err = instances[1].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{&actions.Transfer{
					To:    addr2,
					Value: 201,
				}},
				factory,
			)
			require.NoError(err)
			require.NoError(submit(context.Background()))
			accept2 = expectBlk(instances[1])
		})

		ginkgo.By("clear processing tip", func() {
			results := accept(true)
			require.Len(results, 1)
			require.True(results[0].Success)
			results = accept2(true)
			require.Len(results, 1)
			require.True(results[0].Success)
		})
	})

	ginkgo.It("ensure mempool works", func() {
		ginkgo.By("fail Gossip TransferTx to a stale node when missing previous blocks", func() {
			parser, err := instances[1].lcli.Parser(context.Background())
			require.NoError(err)
			submit, _, _, err := instances[1].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{&actions.Transfer{
					To:    addr2,
					Value: 203,
				}},
				factory,
			)
			require.NoError(err)
			require.NoError(submit(context.Background()))

			err = instances[1].vm.Gossiper().Force(context.TODO())
			require.NoError(err)

			// mempool in 0 should be 1 (old amount), since gossip/submit failed
			require.Equal(instances[0].vm.Mempool().Len(context.TODO()), 1)
		})
	})

	ginkgo.It("ensure unprocessed tip works", func() {
		ginkgo.By("import accepted blocks to instance 2", func() {
			ctx := context.TODO()

			require.Equal(blocks[0].Height(), uint64(1))

			n := instances[2]
			blk1, err := n.vm.ParseBlock(ctx, blocks[0].Bytes())
			require.NoError(err)
			err = blk1.Verify(ctx)
			require.NoError(err)

			// Parse tip
			blk2, err := n.vm.ParseBlock(ctx, blocks[1].Bytes())
			require.NoError(err)
			blk3, err := n.vm.ParseBlock(ctx, blocks[2].Bytes())
			require.NoError(err)

			// Verify tip
			err = blk2.Verify(ctx)
			require.NoError(err)
			err = blk3.Verify(ctx)
			require.NoError(err)

			// Accept tip
			err = blk1.Accept(ctx)
			require.NoError(err)
			err = blk2.Accept(ctx)
			require.NoError(err)
			err = blk3.Accept(ctx)
			require.NoError(err)

			// Parse another
			blk4, err := n.vm.ParseBlock(ctx, blocks[3].Bytes())
			require.NoError(err)
			err = blk4.Verify(ctx)
			require.NoError(err)
			err = blk4.Accept(ctx)
			require.NoError(err)
			require.NoError(n.vm.SetPreference(ctx, blk4.ID()))
		})
	})

	ginkgo.It("processes valid index transactions (w/block listening)", func() {
		// Clear previous txs on instance 0
		accept := expectBlk(instances[0])
		accept(false) // don't care about results

		// Subscribe to blocks
		cli, err := rpc.NewWebSocketClient(instances[0].WebSocketServer.URL, rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
		require.NoError(err)
		require.NoError(cli.RegisterBlocks())

		// Wait for message to be sent
		time.Sleep(2 * pubsub.MaxMessageWait)

		// Fetch balances
		balance, err := instances[0].lcli.Balance(context.TODO(), addrStr)
		require.NoError(err)

		// Send tx
		other, err := ed25519.GeneratePrivateKey()
		require.NoError(err)
		transfer := []chain.Action{&actions.Transfer{
			To:    auth.NewED25519Address(other.PublicKey()),
			Value: 1,
		}}

		parser, err := instances[0].lcli.Parser(context.Background())
		require.NoError(err)
		submit, _, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			transfer,
			factory,
		)
		require.NoError(err)
		require.NoError(submit(context.Background()))

		require.NoError(err)
		accept = expectBlk(instances[0])
		results := accept(false)
		require.Len(results, 1)
		require.True(results[0].Success)

		// Read item from connection
		blk, lresults, prices, err := cli.ListenBlock(context.TODO(), parser)
		require.NoError(err)
		require.Len(blk.Txs, 1)
		tx := blk.Txs[0].Actions[0].(*actions.Transfer)
		require.Equal(tx.Value, uint64(1))
		require.Equal(lresults, results)
		require.Equal(prices, fees.Dimensions{1, 1, 1, 1, 1})

		// Check balance modifications are correct
		balancea, err := instances[0].lcli.Balance(context.TODO(), addrStr)
		require.NoError(err)
		require.Equal(balance, balancea+lresults[0].Fee+1)

		// Close connection when done
		require.NoError(cli.Close())
	})

	ginkgo.It("processes valid index transactions (w/streaming verification)", func() {
		// Create streaming client
		cli, err := rpc.NewWebSocketClient(instances[0].WebSocketServer.URL, rpc.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
		require.NoError(err)

		// Create tx
		other, err := ed25519.GeneratePrivateKey()
		require.NoError(err)
		transfer := []chain.Action{&actions.Transfer{
			To:    auth.NewED25519Address(other.PublicKey()),
			Value: 1,
		}}
		parser, err := instances[0].lcli.Parser(context.Background())
		require.NoError(err)
		_, tx, _, err := instances[0].cli.GenerateTransaction(
			context.Background(),
			parser,
			transfer,
			factory,
		)
		require.NoError(err)

		// Submit tx and accept block
		require.NoError(cli.RegisterTx(tx))

		// Wait for message to be sent
		time.Sleep(2 * pubsub.MaxMessageWait)

		for instances[0].vm.Mempool().Len(context.TODO()) == 0 {
			// We need to wait for mempool to be populated because issuance will
			// return as soon as bytes are on the channel.
			hutils.Outf("{{yellow}}waiting for mempool to return non-zero txs{{/}}\n")
			time.Sleep(500 * time.Millisecond)
		}
		require.NoError(err)
		accept := expectBlk(instances[0])
		results := accept(false)
		require.Len(results, 1)
		require.True(results[0].Success)

		// Read decision from connection
		txID, dErr, result, err := cli.ListenTx(context.TODO())
		require.NoError(err)
		require.Equal(txID, tx.ID())
		require.Nil(dErr)
		require.True(result.Success)
		require.Equal(result, results[0])

		// Close connection when done
		require.NoError(cli.Close())
	})

	ginkgo.It("sends tokens between ed25519 and secp256r1 addresses", func() {
		r1priv, err := secp256r1.GeneratePrivateKey()
		require.NoError(err)
		r1pk := r1priv.PublicKey()
		r1factory := auth.NewSECP256R1Factory(r1priv)
		r1addr := auth.NewSECP256R1Address(r1pk)

		ginkgo.By("send to secp256r1", func() {
			parser, err := instances[0].lcli.Parser(context.Background())
			require.NoError(err)
			submit, _, _, err := instances[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{&actions.Transfer{
					To:    r1addr,
					Value: 2000,
				}},
				factory,
			)
			require.NoError(err)
			require.NoError(submit(context.Background()))
			accept := expectBlk(instances[0])
			results := accept(false)
			require.Len(results, 1)
			require.True(results[0].Success)

			balance, err := instances[0].lcli.Balance(context.TODO(), codec.MustAddressBech32(lconsts.HRP, r1addr))
			require.NoError(err)
			require.Equal(balance, uint64(2000))
		})

		ginkgo.By("send back to ed25519", func() {
			parser, err := instances[0].lcli.Parser(context.Background())
			require.NoError(err)
			submit, _, _, err := instances[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{&actions.Transfer{
					To:    addr,
					Value: 100,
				}},
				r1factory,
			)
			require.NoError(err)
			require.NoError(submit(context.Background()))
			accept := expectBlk(instances[0])
			results := accept(false)
			require.Len(results, 1)
			require.True(results[0].Success)
		})
	})

	ginkgo.It("sends tokens between ed25519 and bls addresses", func() {
		r1priv, err := hbls.GeneratePrivateKey()
		require.NoError(err)
		r1pk := hbls.PublicFromPrivateKey(r1priv)
		r1factory := auth.NewBLSFactory(r1priv)
		r1addr := auth.NewBLSAddress(r1pk)

		ginkgo.By("send to bls", func() {
			parser, err := instances[0].lcli.Parser(context.Background())
			require.NoError(err)
			submit, _, _, err := instances[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{&actions.Transfer{
					To:    r1addr,
					Value: 2000,
				}},
				factory,
			)
			require.NoError(err)
			require.NoError(submit(context.Background()))
			accept := expectBlk(instances[0])
			results := accept(false)
			require.Len(results, 1)
			require.True(results[0].Success)

			balance, err := instances[0].lcli.Balance(context.TODO(), codec.MustAddressBech32(lconsts.HRP, r1addr))
			require.NoError(err)
			require.Equal(balance, uint64(2000))
		})

		ginkgo.By("send back to ed25519 (in separate actions)", func() {
			bbalance, err := instances[0].lcli.Balance(context.TODO(), codec.MustAddressBech32(lconsts.HRP, addr))
			require.NoError(err)

			parser, err := instances[0].lcli.Parser(context.Background())
			require.NoError(err)
			submit, _, _, err := instances[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{
					&actions.Transfer{
						To:    addr,
						Value: 75,
					},
					&actions.Transfer{
						To:    addr,
						Value: 25,
					},
				},
				r1factory,
			)
			require.NoError(err)
			require.NoError(submit(context.Background()))
			accept := expectBlk(instances[0])
			results := accept(false)
			require.Len(results, 1)
			require.True(results[0].Success)

			balance, err := instances[0].lcli.Balance(context.TODO(), codec.MustAddressBech32(lconsts.HRP, addr))
			require.NoError(err)
			require.Equal(balance, bbalance+100)
		})
	})
})

func expectBlk(i instance) func(bool) []*chain.Result {
	require := require.New(ginkgo.GinkgoT())

	ctx := context.TODO()

	// manually signal ready
	require.NoError(i.vm.Builder().Force(ctx))
	// manually ack ready sig as in engine
	<-i.toEngine

	blk, err := i.vm.BuildBlock(ctx)
	require.NoError(err)
	require.NotNil(blk)

	require.NoError(blk.Verify(ctx))
	require.Equal(blk.Status(), choices.Processing)

	err = i.vm.SetPreference(ctx, blk.ID())
	require.NoError(err)

	return func(add bool) []*chain.Result {
		require.NoError(blk.Accept(ctx))
		require.Equal(blk.Status(), choices.Accepted)

		if add {
			blocks = append(blocks, blk)
		}

		lastAccepted, err := i.vm.LastAccepted(ctx)
		require.NoError(err)
		require.Equal(lastAccepted, blk.ID())
		return blk.(*chain.StatelessBlock).Results()
	}
}

var _ common.AppSender = &appSender{}

type appSender struct {
	next      int
	instances []instance
}

func (app *appSender) SendAppGossip(ctx context.Context, _ common.SendConfig, appGossipBytes []byte) error {
	n := len(app.instances)
	sender := app.instances[app.next].nodeID
	app.next++
	app.next %= n
	return app.instances[app.next].vm.AppGossip(ctx, sender, appGossipBytes)
}

func (*appSender) SendAppRequest(context.Context, set.Set[ids.NodeID], uint32, []byte) error {
	return nil
}

func (*appSender) SendAppError(context.Context, ids.NodeID, uint32, int32, string) error {
	return nil
}

func (*appSender) SendAppResponse(context.Context, ids.NodeID, uint32, []byte) error {
	return nil
}

func (*appSender) SendCrossChainAppRequest(context.Context, ids.ID, uint32, []byte) error {
	return nil
}

func (*appSender) SendCrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
	return nil
}

func (*appSender) SendCrossChainAppError(context.Context, ids.ID, uint32, int32, string) error {
	return nil
}

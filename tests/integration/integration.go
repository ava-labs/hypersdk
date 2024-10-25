// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/enginetest"
	"github.com/ava-labs/avalanchego/snow/validators/validatorstest"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/grpcutils"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/abi"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/extension/externalsubscriber"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/pubsub"
	"github.com/ava-labs/hypersdk/tests/registry"
	"github.com/ava-labs/hypersdk/tests/workload"
	"github.com/ava-labs/hypersdk/vm"

	pb "github.com/ava-labs/hypersdk/proto/pb/externalsubscriber"
	hutils "github.com/ava-labs/hypersdk/utils"
	ginkgo "github.com/onsi/ginkgo/v2"
)

// TODO integration tests require MinBlockGap to be 0, so that BuildBlock can be called
// immediately after issuing a tx. After https://github.com/ava-labs/hypersdk/issues/1217, switch
// integration/e2e tests to set the parameters and only allow the VM to populate VM-specific parameters.

const numVMs = 3

var (
	logFactory logging.Factory
	log        logging.Logger

	// when used with embedded VMs
	instances            []*instance
	sendAppGossipCounter int
	uris                 []string
	blocks               []snowman.Block

	networkID uint32

	// Injected values populated by Setup
	createVM      func(...vm.Option) (*vm.VM, error)
	networkConfig workload.TestNetworkConfiguration
	vmID          ids.ID

	txWorkload  workload.TxWorkload
	authFactory chain.AuthFactory

	externalSubscriberAcceptedBlocksCh chan ids.ID
)

type instance struct {
	chainID         ids.ID
	nodeID          ids.NodeID
	vm              *vm.VM
	toEngine        chan common.Message
	routerServer    *httptest.Server
	JSONRPCServer   *httptest.Server
	WebSocketServer *httptest.Server
	cli             *jsonrpc.JSONRPCClient // clients for embedded VMs
	onAccept        func(snowman.Block)
}

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

func Setup(
	newVM func(...vm.Option) (*vm.VM, error),
	networkConfigImpl workload.TestNetworkConfiguration,
	id ids.ID,
	generator workload.TxGenerator,
	authF chain.AuthFactory,
) {
	createVM = newVM
	networkConfig = networkConfigImpl
	vmID = id
	txWorkload = workload.TxWorkload{
		Generator: generator,
	}
	authFactory = authF

	setInstances()
}

func setInstances() {
	require := require.New(ginkgo.GinkgoT())

	log.Info("VMID", zap.Stringer("id", vmID))

	// create embedded VMs
	instances = make([]*instance, numVMs)

	createParserFromBytes := func(_ []byte) (chain.Parser, error) {
		return networkConfig.Parser(), nil
	}

	externalSubscriberAcceptedBlocksCh = make(chan ids.ID, 1)
	externalSubscriber0 := externalsubscriber.NewExternalSubscriberServer(log, createParserFromBytes, []event.Subscription[*chain.ExecutedBlock]{
		event.SubscriptionFunc[*chain.ExecutedBlock]{
			AcceptF: func(blk *chain.ExecutedBlock) error {
				externalSubscriberAcceptedBlocksCh <- blk.BlockID
				return nil
			},
		},
	})

	listener, err := grpcutils.NewListener()
	require.NoError(err)
	serverCloser := grpcutils.ServerCloser{}

	server := grpcutils.NewServer()
	pb.RegisterExternalSubscriberServer(server, externalSubscriber0)
	serverCloser.Add(server)

	go grpcutils.Serve(listener, server)

	ginkgo.DeferCleanup(func() {
		serverCloser.Stop()
		_ = listener.Close()
	})

	vmWithExternalSubscriberConfig := vm.NewConfig()
	actualExternalSubscriberConfig := externalsubscriber.Config{
		Enabled:       true,
		ServerAddress: listener.Addr().String(),
	}
	actualExternalSubscriberConfigBytes, err := json.Marshal(actualExternalSubscriberConfig)
	require.NoError(err)
	namespacedConfig := map[string]json.RawMessage{
		externalsubscriber.Namespace: actualExternalSubscriberConfigBytes,
	}
	vmWithExternalSubscriberConfig.ServiceConfig = namespacedConfig

	externalSubscriberConfigBytes, err := json.Marshal(vmWithExternalSubscriberConfig)
	require.NoError(err)
	configs := make([][]byte, numVMs)
	configs[0] = externalSubscriberConfigBytes

	networkID = networkConfig.Parser().Rules(0).GetNetworkID()
	subnetID := ids.GenerateTestID()
	chainID := networkConfig.Parser().Rules(0).GetChainID()

	app := &enginetest.Sender{
		SendAppGossipF: func(ctx context.Context, _ common.SendConfig, appGossipBytes []byte) error {
			n := len(instances)
			sender := instances[sendAppGossipCounter].nodeID
			sendAppGossipCounter++
			sendAppGossipCounter %= n
			return instances[sendAppGossipCounter].vm.AppGossip(ctx, sender, appGossipBytes)
		},
	}
	for i := range instances {
		nodeID := ids.GenerateTestNodeID()
		sk, err := bls.NewSecretKey()
		require.NoError(err)
		l, err := logFactory.Make(nodeID.String())
		require.NoError(err)
		dname, err := os.MkdirTemp("", nodeID.String()+"-chainData")
		require.NoError(err)
		ginkgo.DeferCleanup(func() {
			os.RemoveAll(dname)
		})
		snowCtx := &snow.Context{
			NetworkID:      networkID,
			SubnetID:       subnetID,
			ChainID:        chainID,
			NodeID:         nodeID,
			Log:            l,
			ChainDataDir:   dname,
			Metrics:        metrics.NewPrefixGatherer(),
			PublicKey:      bls.PublicFromSecretKey(sk),
			ValidatorState: &validatorstest.State{},
		}

		toEngine := make(chan common.Message, 1)
		db := memdb.New()

		v, err := createVM(
			vm.WithManual(),
		)
		require.NoError(err)
		require.NoError(v.Initialize(
			context.TODO(),
			snowCtx,
			db,
			networkConfig.GenesisBytes(),
			nil,
			configs[i],
			toEngine,
			nil,
			app,
		))

		hd, err := v.CreateHandlers(context.TODO())
		require.NoError(err)

		router := http.NewServeMux()
		for endpoint, handler := range hd {
			router.Handle(endpoint, handler)
		}

		routerServer := httptest.NewServer(router)
		jsonRPCServer := httptest.NewServer(hd[jsonrpc.Endpoint])
		webSocketServer := httptest.NewServer(hd[ws.Endpoint])
		instances[i] = &instance{
			chainID:         snowCtx.ChainID,
			nodeID:          snowCtx.NodeID,
			vm:              v,
			toEngine:        toEngine,
			routerServer:    routerServer,
			JSONRPCServer:   jsonRPCServer,
			WebSocketServer: webSocketServer,
			cli:             jsonrpc.NewJSONRPCClient(jsonRPCServer.URL),
		}

		// Force sync ready (to mimic bootstrapping from genesis)
		v.ForceReady()

		// Expect external subscriber to receive a notification when instance 0 accepts a block
		instances[0].onAccept = func(blk snowman.Block) {
			select {
			case externalReceivedBlkID := <-externalSubscriberAcceptedBlocksCh:
				require.Equal(blk.ID(), externalReceivedBlkID)
			case <-time.After(5 * time.Second):
				require.Fail("external subscriber did not receive accepted block")
			}
		}
	}

	uris = make([]string, len(instances))
	for i, inst := range instances {
		uris[i] = inst.routerServer.URL
	}

	blocks = []snowman.Block{}

	log.Info("created instances", zap.Int("count", len(instances)))
}

var _ = ginkgo.AfterSuite(func() {
	require := require.New(ginkgo.GinkgoT())

	for _, iv := range instances {
		iv.routerServer.Close()
		iv.JSONRPCServer.Close()
		iv.WebSocketServer.Close()
		require.NoError(iv.vm.Shutdown(context.TODO()))
	}
})

var _ = ginkgo.Describe("[HyperSDK APIs]", func() {
	ctx := context.Background()
	require := require.New(ginkgo.GinkgoT())

	ginkgo.It("Ping", func() {
		ginkgo.By("Send Ping request to every node")
		workload.Ping(ctx, require, uris)
	})
	ginkgo.It("GetNetwork", func() {
		ginkgo.By("Send GetNetwork request to every node")
		workload.GetNetwork(ctx, require, uris, instances[0].vm.NetworkID(), instances[0].chainID)
	})
	ginkgo.It("GetABI", func() {
		ginkgo.By("Gets ABI")

		actionCodec, outputCodec := instances[0].vm.ActionCodec(), instances[0].vm.OutputCodec()
		expectedABI, err := abi.NewABI(actionCodec.GetRegisteredTypes(), (*outputCodec).GetRegisteredTypes())
		require.NoError(err)

		workload.GetABI(ctx, require, uris, expectedABI)
	})
})

var _ = ginkgo.Describe("[Tx Processing]", ginkgo.Serial, func() {
	ctx := context.Background()
	require := require.New(ginkgo.GinkgoT())

	ginkgo.It("get currently accepted block ID", func() {
		for _, inst := range instances {
			cli := inst.cli
			_, _, _, err := cli.Accepted(context.Background())
			require.NoError(err)
		}
	})

	var (
		initialTx          *chain.Transaction
		initialTxAssertion workload.TxAssertion
	)
	ginkgo.It("Gossip TransferTx to a different node", func() {
		uri := uris[0]
		ginkgo.By("issue TransferTx", func() {
			tx, assertion, err := txWorkload.Generator.GenerateTx(ctx, uri)
			initialTxAssertion = assertion
			initialTx = tx
			require.NoError(err)
			_, err = instances[0].cli.SubmitTx(ctx, initialTx.Bytes())
			require.NoError(err)

			require.Equal(1, instances[0].vm.Mempool().Len(context.Background()))
		})

		ginkgo.By("skip mempool duplicate", func() {
			_, err := instances[0].cli.SubmitTx(
				context.Background(),
				initialTx.Bytes(),
			)
			require.ErrorContains(err, vm.ErrNotAdded.Error()) //nolint:forbidigo
		})

		ginkgo.By("send gossip from node 0 to 1", func() {
			require.NoError(instances[0].vm.Gossiper().Force(ctx))
		})

		ginkgo.By("skip invalid time", func() {
			tx := chain.NewTxData(
				&chain.Base{
					ChainID:   instances[0].chainID,
					Timestamp: 1,
					MaxFee:    1000,
				},
				initialTx.Actions,
			)
			// Must do manual construction to avoid `tx.Sign` error (would fail with
			// 0 timestamp)
			unsignedTxBytes, err := tx.UnsignedBytes()
			require.NoError(err)
			auth, err := authFactory.Sign(unsignedTxBytes)
			require.NoError(err)
			signedTx := chain.Transaction{
				TransactionData: *tx,
				Auth:            auth,
			}
			p := codec.NewWriter(0, consts.MaxInt) // test codec growth
			require.NoError(signedTx.Marshal(p))
			require.NoError(p.Err())
			_, err = instances[0].cli.SubmitTx(
				context.Background(),
				p.Bytes(),
			)
			require.ErrorContains(err, chain.ErrMisalignedTime.Error()) //nolint:forbidigo
		})

		ginkgo.By("skip duplicate (after gossip, which shouldn't clear)", func() {
			_, err := instances[0].cli.SubmitTx(
				context.Background(),
				initialTx.Bytes(),
			)
			require.ErrorContains(err, vm.ErrNotAdded.Error()) //nolint:forbidigo
		})

		ginkgo.By("receive gossip in the node 1, and signal block build", func() {
			require.NoError(instances[1].vm.Builder().Force(ctx))
			<-instances[1].toEngine
		})

		ginkgo.By("build block in the node 1", func() {
			blk, err := instances[1].vm.BuildBlock(ctx)
			require.NoError(err)

			require.NoError(blk.Verify(ctx))

			require.NoError(instances[1].vm.SetPreference(ctx, blk.ID()))

			require.NoError(blk.Accept(ctx))
			blocks = append(blocks, blk)

			lastAccepted, err := instances[1].vm.LastAccepted(ctx)
			require.NoError(err)
			require.Equal(lastAccepted, blk.ID())

			results := blk.(*chain.StatefulBlock).Results()
			require.Len(results, 1)
			require.True(results[0].Success)
		})

		ginkgo.By("ensure balance is updated", func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()

			initialTxAssertion(ctx, require, uris[1])
		})
	})

	ginkgo.It("ensure multiple txs work ", func() {
		ginkgo.By("transfer funds again", func() {
			tx, txAssertion, err := txWorkload.Generator.GenerateTx(ctx, uris[0])

			require.NoError(err)
			_, err = instances[1].cli.SubmitTx(ctx, tx.Bytes())
			require.NoError(err)

			accept := expectBlk(instances[1])
			results := accept(true)
			require.Len(results, 1)

			ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()
			txAssertion(ctx, require, uris[1])
		})

		ginkgo.By("transfer funds again (test storage keys)", func() {
			for i := 0; i < 4; i++ {
				tx, _, err := txWorkload.Generator.GenerateTx(ctx, uris[0])

				require.NoError(err)
				_, err = instances[1].cli.SubmitTx(ctx, tx.Bytes())
				require.NoError(err)
			}

			// Ensure we can handle case where accepted block is not processed
			latestBlock := blocks[len(blocks)-1]
			latestBlock.(*chain.StatefulBlock).MarkUnprocessed()

			// Accept new block (should use accepted state)
			accept := expectBlk(instances[1])
			results := accept(true)

			// Check results
			require.Len(results, 4)
			for i := 0; i < 4; i++ {
				require.True(results[i].Success)
			}
		})
	})

	ginkgo.It("Test processing block handling", func() {
		var accept, accept2 func(bool) []*chain.Result
		ginkgo.By("create processing tip", func() {
			tx, _, err := txWorkload.Generator.GenerateTx(ctx, uris[0])
			require.NoError(err)
			_, err = instances[1].cli.SubmitTx(ctx, tx.Bytes())
			require.NoError(err)

			accept = expectBlk(instances[1])

			tx, _, err = txWorkload.Generator.GenerateTx(ctx, uris[0])
			require.NoError(err)
			_, err = instances[1].cli.SubmitTx(ctx, tx.Bytes())
			require.NoError(err)
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

		ginkgo.By("Confirm accepted blocks indexed", func() {
			workload.GetBlocks(ctx, require, networkConfig.Parser(), []string{uris[1]})
		})
	})

	ginkgo.It("ensure mempool works", func() {
		ginkgo.By("fail Gossip TransferTx to a stale node when missing previous blocks", func() {
			tx, _, err := txWorkload.Generator.GenerateTx(ctx, uris[0])
			require.NoError(err)

			_, err = instances[1].cli.SubmitTx(ctx, tx.Bytes())
			require.NoError(err)

			require.NoError(instances[1].vm.Gossiper().Force(ctx))

			// mempool in 0 should be 1 (old amount), since gossip/submit failed
			require.Equal(1, instances[0].vm.Mempool().Len(context.TODO()))
		})
	})

	ginkgo.It("ensure unprocessed tip works", func() {
		ginkgo.By("import accepted blocks to instance 2", func() {
			ctx := context.TODO()
			require.Equal(uint64(1), blocks[0].Height())

			n := instances[2]
			blk1, err := n.vm.ParseBlock(ctx, blocks[0].Bytes())
			require.NoError(err)
			require.NoError(blk1.Verify(ctx))

			// Parse tip
			blk2, err := n.vm.ParseBlock(ctx, blocks[1].Bytes())
			require.NoError(err)
			blk3, err := n.vm.ParseBlock(ctx, blocks[2].Bytes())
			require.NoError(err)

			// Verify tip
			require.NoError(blk2.Verify(ctx))
			require.NoError(blk3.Verify(ctx))

			// Accept tip
			require.NoError(blk1.Accept(ctx))
			require.NoError(blk2.Accept(ctx))
			require.NoError(blk3.Accept(ctx))

			// Parse another
			blk4, err := n.vm.ParseBlock(ctx, blocks[3].Bytes())
			require.NoError(err)
			require.NoError(blk4.Verify(ctx))
			require.NoError(blk4.Accept(ctx))
			require.NoError(n.vm.SetPreference(ctx, blk4.ID()))
		})
	})

	ginkgo.It("processes valid index transactions (w/block listening)", func() {
		// Clear previous txs on instance 0
		accept := expectBlk(instances[0])
		accept(false) // don't care about results

		// Subscribe to blocks
		cli, err := ws.NewWebSocketClient(instances[0].WebSocketServer.URL, ws.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
		require.NoError(err)
		require.NoError(cli.RegisterBlocks())

		// Wait for message to be sent
		time.Sleep(2 * pubsub.MaxMessageWait) // TODO: remove after websocket server rewrite

		tx, txAssertion, err := txWorkload.Generator.GenerateTx(ctx, uris[0])
		require.NoError(err)

		_, err = instances[0].cli.SubmitTx(ctx, tx.Bytes())
		require.NoError(err)

		accept = expectBlk(instances[0])
		results := accept(false)
		require.Len(results, 1)
		require.True(results[0].Success)

		cctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		txAssertion(cctx, require, uris[0])

		// Read item from connection
		blk, lresults, prices, err := cli.ListenBlock(context.TODO(), networkConfig.Parser())
		require.NoError(err)
		require.Len(blk.Txs, 1)
		require.Equal(lresults, results)
		require.Equal(fees.Dimensions{100, 100, 100, 100, 100}, prices)

		// Close connection when done
		require.NoError(cli.Close())
	})

	ginkgo.It("processes valid index transactions (w/streaming verification)", func() {
		// Create streaming client
		cli, err := ws.NewWebSocketClient(instances[0].WebSocketServer.URL, ws.DefaultHandshakeTimeout, pubsub.MaxPendingMessages, pubsub.MaxReadMessageSize)
		require.NoError(err)

		// Create tx
		tx, txAssertion, err := txWorkload.Generator.GenerateTx(ctx, uris[0])
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
		accept := expectBlk(instances[0])
		results := accept(false)
		require.Len(results, 1)
		require.True(results[0].Success)

		cctx, cancel := context.WithTimeout(ctx, 1*time.Second)
		defer cancel()
		txAssertion(cctx, require, uris[0])

		// Read decision from connection
		txID, dErr, result, err := cli.ListenTx(context.TODO())
		require.NoError(err)
		require.Equal(txID, tx.ID())
		require.NoError(dErr)
		require.True(result.Success)
		require.Equal(result, results[0])

		// Close connection when done
		require.NoError(cli.Close())
	})
})

var _ = ginkgo.Describe("[Custom VM Tests]", func() {
	require := require.New(ginkgo.GinkgoT())

	for testRegistry := range registry.GetTestsRegistries() {
		for _, test := range testRegistry.List() {
			ginkgo.It(test.Name, func() {
				testNetwork := &Network{uris: uris}
				require.NoError(test.Fnc(ginkgo.GinkgoT(), testNetwork), "Test %s failed with an error", test.Name)
			})
		}
	}
})

func expectBlk(i *instance) func(add bool) []*chain.Result {
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

	require.NoError(i.vm.SetPreference(ctx, blk.ID()))

	return func(add bool) []*chain.Result {
		require.NoError(blk.Accept(ctx))
		if i.onAccept != nil {
			i.onAccept(blk)
		}

		if add {
			blocks = append(blocks, blk)
		}

		lastAccepted, err := i.vm.LastAccepted(ctx)
		require.NoError(err)
		require.Equal(lastAccepted, blk.ID())
		return blk.(*chain.StatefulBlock).Results()
	}
}

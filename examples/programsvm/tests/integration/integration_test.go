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
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/programsvm/actions"
	"github.com/ava-labs/hypersdk/examples/programsvm/controller"
	"github.com/ava-labs/hypersdk/examples/programsvm/genesis"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/vm"

	auth "github.com/ava-labs/hypersdk/auth"
	lconsts "github.com/ava-labs/hypersdk/examples/programsvm/consts"
	lrpc "github.com/ava-labs/hypersdk/examples/programsvm/rpc"
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
			Metrics:        metrics.NewPrefixGatherer(),
			PublicKey:      bls.PublicFromSecretKey(sk),
			ValidatorState: &validators.TestState{},
		}

		toEngine := make(chan common.Message, 1)
		db := memdb.New()

		v, err := controller.New(vm.WithManualGossiper(), vm.WithManualBuilder())
		require.NoError(err)
		require.NoError(v.Initialize(
			context.TODO(),
			snowCtx,
			db,
			genesisBytes,
			nil,
			[]byte(
				`{
				  "config": {
				    "logLevel":"debug"
				  }
				}`,
			),
			toEngine,
			nil,
			app,
		))

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
		require.NoError(iv.vm.Shutdown(context.TODO()))
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
	ginkgo.It("test publish program", func() {
		var result []byte
		parser, err := instances[0].lcli.Parser(context.TODO())
		require.NoError(err)
		ginkgo.By("issue publish to the first node", func() {

			// Generate transaction
			bytes, err := os.ReadFile("./simple.wasm")
			require.NoError(err)

			submit, _, _, err := instances[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{&actions.PublishProgram{
					ProgramBytes: bytes,
				}},
				factory,
			)
			require.NoError(err)

			// Broadcast and wait for transaction
			require.NoError(submit(context.Background()))

			accept := expectBlk(instances[0])
			results := accept(false)
			require.Len(results, 1)
			require.True(results[0].Success)

			result = results[0].Outputs[0][0]

		})
		ginkgo.By("issue deploy to the first node", func() {
			submit, _, _, err := instances[0].cli.GenerateTransaction(
				context.Background(),
				parser,
				[]chain.Action{&actions.DeployProgram{
					ProgramID:    result,
					CreationInfo: []byte{1, 2, 3, 4, 5, 6},
				}},
				factory,
			)
			require.NoError(err)

			// Broadcast and wait for transaction
			require.NoError(submit(context.Background()))

			accept := expectBlk(instances[0])
			results := accept(false)
			require.Len(results, 1)
			require.True(results[0].Success)
			result = results[0].Outputs[0][0]
		})

		ginkgo.By("issue call Program to the first node", func() {
			simulatedAction := actions.CallProgram{
				Program:  codec.Address(result),
				Function: "get_value",
			}

			specifiedStateKeysSet, fuel, err := instances[0].lcli.Simulate(context.Background(), simulatedAction, codec.EmptyAddress)
			specifiedStateKeys := make([]actions.StateKeyPermission, 0, len(specifiedStateKeysSet))
			for key, value := range specifiedStateKeysSet {
				specifiedStateKeys = append(specifiedStateKeys, actions.StateKeyPermission{Key: key, Permission: value})
			}
			require.NoError(err)
			start := time.Now()
			for j := 0; j < 100; j++ {
				for i := 0; i < 10; i++ {
					require.NoError(err)
					valueAmount := uint64(10000000 * (i + 10*j))

					submit, _, _, err := instances[0].cli.GenerateTransaction(
						context.Background(),
						parser,
						[]chain.Action{&actions.CallProgram{
							Program:            codec.Address(result),
							Function:           "get_value",
							Value:              valueAmount,
							SpecifiedStateKeys: specifiedStateKeys,
							Fuel:               fuel,
						}},
						factory,
					)
					require.NoError(err)

					// Broadcast and wait for transaction
					err = submit(context.Background())
					require.NoError(err)
				}

				accept := expectBlk(instances[0])
				results := accept(false)
				require.Len(results, 10)
				require.True(results[0].Success)
			}
			end := time.Now()
			require.Greater(end, start)
			totalMS := end.Sub(start).Milliseconds()
			require.Less(totalMS, int64(1500))
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

	require.NoError(i.vm.SetPreference(ctx, blk.ID()))

	return func(add bool) []*chain.Result {
		require.NoError(blk.Accept(ctx))

		if add {
			blocks = append(blocks, blk)
		}

		lastAccepted, err := i.vm.LastAccepted(ctx)
		require.NoError(err)
		require.Equal(lastAccepted, blk.ID())
		return blk.(*chain.StatelessBlock).Results()
	}
}

var _ common.AppSender = (*appSender)(nil)

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

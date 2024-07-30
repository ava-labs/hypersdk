// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ava-labs/avalanchego/api/metrics"
	"github.com/ava-labs/avalanchego/database/memdb"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/consensus/snowman"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/fatih/color"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/examples/lineagevm/auth"
	"github.com/ava-labs/hypersdk/examples/lineagevm/controller"
	"github.com/ava-labs/hypersdk/examples/lineagevm/genesis"
	"github.com/ava-labs/hypersdk/fees"
	"github.com/ava-labs/hypersdk/rpc"

	lconsts "github.com/ava-labs/hypersdk/examples/lineagevm/consts"
	lrpc "github.com/ava-labs/hypersdk/examples/lineagevm/rpc"
	ginkgo "github.com/onsi/ginkgo/v2"
)

type prepareResult struct {
	instance instance
	factory  *auth.ED25519Factory
}

func prepare(t *testing.T) prepareResult {
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

	return prepareResult{
		instance: instances[0],
		factory:  factory,
	}
}

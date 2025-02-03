// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/grpcutils"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/api/ws"
	"github.com/ava-labs/hypersdk/auth"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/chain/chaintest"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/crypto"
	"github.com/ava-labs/hypersdk/crypto/ed25519"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/extension/externalsubscriber"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/internal/mempool"
	"github.com/ava-labs/hypersdk/internal/validitywindow"
	"github.com/ava-labs/hypersdk/keys"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/balance"
	"github.com/ava-labs/hypersdk/state/metadata"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/vm/defaultvm"
	"github.com/ava-labs/hypersdk/vm/vmtest"

	avasnow "github.com/ava-labs/avalanchego/snow"
	stateapi "github.com/ava-labs/hypersdk/api/state"
	hcontext "github.com/ava-labs/hypersdk/context"
	pb "github.com/ava-labs/hypersdk/proto/pb/externalsubscriber"
)

type VMTestNetworkOptions struct {
	AllocationOverride  func() []uint64
	GenesisRuleOverride func(*genesis.Rules)
	ConfigBytes         func() []byte
}

type VMTestNetworkOption func(*VMTestNetworkOptions)

func NewVMTestNetworkOptions(opts ...VMTestNetworkOption) *VMTestNetworkOptions {
	options := &VMTestNetworkOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

func WithAllocationOverride(f func() []uint64) func(*VMTestNetworkOptions) {
	return func(opts *VMTestNetworkOptions) {
		opts.AllocationOverride = f
	}
}

func WithGenesisRulesOverride(f func(*genesis.Rules)) func(*VMTestNetworkOptions) {
	return func(opts *VMTestNetworkOptions) {
		opts.GenesisRuleOverride = f
	}
}

func WithConfigBytes(f func() []byte) func(*VMTestNetworkOptions) {
	return func(opts *VMTestNetworkOptions) {
		opts.ConfigBytes = f
	}
}

func NewTestVMFactory(r *require.Assertions) *vm.Factory {
	var (
		actionParser = codec.NewTypeParser[chain.Action]()
		authParser   = codec.NewTypeParser[chain.Auth]()
		outputParser = codec.NewTypeParser[codec.Typed]()
	)
	r.NoError(errors.Join(
		actionParser.Register(&chaintest.TestAction{}, nil),
		authParser.Register(&auth.ED25519{}, auth.UnmarshalED25519),
		outputParser.Register(&chaintest.TestOutput{}, nil),
	))
	return vm.NewFactory(
		genesis.DefaultGenesisFactory{},
		balance.NewPrefixBalanceHandler([]byte{0}),
		metadata.NewDefaultManager(),
		actionParser,
		authParser,
		outputParser,
		auth.Engines(),
		append(defaultvm.NewDefaultOptions(), vm.WithManual())...,
	)
}

func NewVMTestNetwork(ctx context.Context, t *testing.T, numVMs int, opts ...VMTestNetworkOption) *vmtest.TestNetwork {
	r := require.New(t)
	options := NewVMTestNetworkOptions(opts...)
	factory := NewTestVMFactory(r)

	var fundRequests []uint64
	if options.AllocationOverride == nil {
		fundRequests = append(fundRequests, uint64(1_000_000_000_000_000))
	} else {
		fundRequests = options.AllocationOverride()
	}

	allocations := make([]*genesis.CustomAllocation, 0, len(fundRequests))
	authFactories := make([]chain.AuthFactory, 0, len(fundRequests))
	for _, fundRequest := range fundRequests {
		privKey, err := ed25519.GeneratePrivateKey()
		r.NoError(err)
		authFactory := auth.NewED25519Factory(privKey)

		authFactories = append(authFactories, authFactory)
		allocations = append(allocations, &genesis.CustomAllocation{
			Address: authFactory.Address(),
			Balance: fundRequest,
		})
	}

	testRules := genesis.NewDefaultRules()
	testRules.MinBlockGap = 0
	testRules.MinEmptyBlockGap = 0
	genesis := &genesis.DefaultGenesis{
		StateBranchFactor: merkledb.BranchFactor16,
		CustomAllocation:  allocations,
		Rules:             testRules,
	}
	// Set test default of MinEmptyBlockGap = 0
	// so we don't need to wait to build a block
	genesis.Rules.MinEmptyBlockGap = 0
	if options.GenesisRuleOverride != nil {
		options.GenesisRuleOverride(genesis.Rules)
	}
	genesisBytes, err := json.Marshal(genesis)
	r.NoError(err)
	var configBytes []byte
	if options.ConfigBytes != nil {
		configBytes = options.ConfigBytes()
	}

	return vmtest.NewTestNetwork(ctx, t, factory, numVMs, authFactories, genesisBytes, nil, configBytes)
}

func TestEmptyBlock(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	network := NewVMTestNetwork(ctx, t, 2)
	network.SetState(ctx, avasnow.NormalOp)
	defer network.Shutdown(ctx)

	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{}))
}

func TestValidBlocks(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	network := NewVMTestNetwork(ctx, t, 2)
	network.SetState(ctx, avasnow.NormalOp)
	defer network.Shutdown(ctx)

	tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.AuthFactories()[0])
	r.NoError(err)
	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx}))

	tx, err = network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
		Nonce:           1,
	}}, network.AuthFactories()[0])
	r.NoError(err)
	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx}))
}

func TestSubmitTx(t *testing.T) {
	tests := []struct {
		name      string
		makeTx    func(r *require.Assertions, network *vmtest.TestNetwork) *chain.Transaction
		targetErr error
	}{
		{
			name: "valid tx",
			makeTx: func(r *require.Assertions, network *vmtest.TestNetwork) *chain.Transaction {
				unsignedTx := chain.NewTxData(
					&chain.Base{
						ChainID:   network.ChainID(),
						Timestamp: utils.UnixRMilli(time.Now().UnixMilli(), 1_000),
						MaxFee:    1_000,
					},
					[]chain.Action{&chaintest.TestAction{NumComputeUnits: 1}},
				)
				tx, err := unsignedTx.Sign(network.AuthFactories()[0])
				r.NoError(err)
				return tx
			},
			targetErr: nil,
		},
		{
			name: validitywindow.ErrMisalignedTime.Error(),
			makeTx: func(r *require.Assertions, network *vmtest.TestNetwork) *chain.Transaction {
				unsignedTx := chain.NewTxData(
					&chain.Base{
						ChainID:   network.ChainID(),
						Timestamp: 1,
						MaxFee:    1_000,
					},
					[]chain.Action{&chaintest.TestAction{NumComputeUnits: 1}},
				)
				tx, err := unsignedTx.Sign(network.AuthFactories()[0])
				r.NoError(err)
				return tx
			},
			targetErr: validitywindow.ErrMisalignedTime,
		},
		{
			name: validitywindow.ErrTimestampExpired.Error(),
			makeTx: func(r *require.Assertions, network *vmtest.TestNetwork) *chain.Transaction {
				unsignedTx := chain.NewTxData(
					&chain.Base{
						ChainID:   network.ChainID(),
						Timestamp: int64(time.Millisecond),
						MaxFee:    1_000,
					},
					[]chain.Action{&chaintest.TestAction{NumComputeUnits: 1}},
				)
				tx, err := unsignedTx.Sign(network.AuthFactories()[0])
				r.NoError(err)
				return tx
			},
			targetErr: validitywindow.ErrTimestampExpired,
		},
		{
			name: validitywindow.ErrFutureTimestamp.Error(),
			makeTx: func(r *require.Assertions, network *vmtest.TestNetwork) *chain.Transaction {
				unsignedTx := chain.NewTxData(
					&chain.Base{
						ChainID:   network.ChainID(),
						Timestamp: utils.UnixRMilli(time.Now().UnixMilli(), time.Hour.Milliseconds()),
						MaxFee:    1_000,
					},
					[]chain.Action{&chaintest.TestAction{NumComputeUnits: 1}},
				)
				tx, err := unsignedTx.Sign(network.AuthFactories()[0])
				r.NoError(err)
				return tx
			},
			targetErr: validitywindow.ErrFutureTimestamp,
		},
		{
			name: "invalid auth",
			makeTx: func(r *require.Assertions, network *vmtest.TestNetwork) *chain.Transaction {
				unsignedTx := chain.NewTxData(
					&chain.Base{
						ChainID:   network.ChainID(),
						Timestamp: utils.UnixRMilli(time.Now().UnixMilli(), 30_000),
						MaxFee:    1_000,
					},
					[]chain.Action{&chaintest.TestAction{NumComputeUnits: 1}},
				)
				invalidAuth, err := network.AuthFactories()[0].Sign([]byte{0})
				r.NoError(err)
				tx, err := chain.NewTransaction(unsignedTx.Base, unsignedTx.Actions, invalidAuth)
				r.NoError(err)
				return tx
			},
			targetErr: crypto.ErrInvalidSignature,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			r := require.New(t)
			network := NewVMTestNetwork(ctx, t, 2)
			network.SetState(ctx, avasnow.NormalOp)
			defer network.Shutdown(ctx)

			invalidTx := test.makeTx(r, network)
			network.ConfirmInvalidTx(ctx, invalidTx, test.targetErr)
		})
	}
}

func TestValidityWindowDuplicateAcceptedBlock(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	network := NewVMTestNetwork(ctx, t, 2)
	network.SetState(ctx, avasnow.NormalOp)
	defer network.Shutdown(ctx)

	tx0, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.AuthFactories()[0])
	r.NoError(err)

	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx0}))

	network.ConfirmInvalidTx(ctx, tx0, chain.ErrDuplicateTx)

	// Build another block, so that the duplicate is in an accepted ancestor
	// instead of the direct parent (last accepted block)
	tx1, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
		Nonce:           1,
	}}, network.AuthFactories()[0])
	r.NoError(err)
	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx1}))

	// The duplicate transaction should still fail
	network.ConfirmInvalidTx(ctx, tx0, chain.ErrDuplicateTx)
}

func TestValidityWindowDuplicateProcessingAncestor(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	network := NewVMTestNetwork(ctx, t, 2)
	network.SetState(ctx, avasnow.NormalOp)
	defer network.Shutdown(ctx)

	tx0, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.AuthFactories()[0])
	r.NoError(err)
	tx1, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
		Nonce:           1,
	}}, network.AuthFactories()[0])
	r.NoError(err)

	txs0 := []*chain.Transaction{tx0}
	network.SubmitTxs(ctx, txs0)
	vmBlk1s := network.BuildBlockAndUpdateHead(ctx)
	network.ConfirmTxsInBlock(ctx, vmBlk1s[0].Output.ExecutionBlock, txs0)

	// Issuing the same tx should fail
	// Note: this test must be modified if we change the semantics of re-issuing a block
	// currently in a processing block to return a nil error
	network.ConfirmInvalidTx(ctx, tx0, chain.ErrDuplicateTx)

	txs1 := []*chain.Transaction{tx1}
	network.SubmitTxs(ctx, txs1)
	vmBlk2s := network.BuildBlockAndUpdateHead(ctx)
	network.ConfirmTxsInBlock(ctx, vmBlk2s[0].Output.ExecutionBlock, txs1)

	// Issuing the same tx (still in a processing ancestor) should fail
	network.ConfirmInvalidTx(ctx, tx0, chain.ErrDuplicateTx)

	for i, blk := range vmBlk1s {
		r.NoError(blk.Accept(ctx), "failed to accept block at VM index %d", i)
	}
	for i, blk := range vmBlk2s {
		r.NoError(blk.Accept(ctx), "failed to accept block at VM index %d", i)
	}
}

func TestIssueDuplicateInMempool(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	network := NewVMTestNetwork(ctx, t, 2)
	network.SetState(ctx, avasnow.NormalOp)
	defer network.Shutdown(ctx)

	tx0, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.AuthFactories()[0])
	r.NoError(err)

	vm0 := network.VMs[0].VM
	r.NoError(vm0.SubmitTx(ctx, tx0))

	// This behavior is not required. We could just as easily return a nil error to report
	// to the user that the tx is in the mempool because it was already issued.
	r.ErrorIs(vm0.SubmitTx(ctx, tx0), vm.ErrNotAdded)
}

func TestForceGossip(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	network := NewVMTestNetwork(ctx, t, 2)
	network.SetState(ctx, avasnow.NormalOp)
	defer network.Shutdown(ctx)

	tx0, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.AuthFactories()[0])
	r.NoError(err)

	vm0 := network.VMs[0].VM
	r.NoError(vm0.SubmitTx(ctx, tx0))
	r.NoError(vm0.Gossiper().Force(ctx))

	vm1 := network.VMs[1].VM
	mempool := vm1.Mempool().(*mempool.Mempool[*chain.Transaction])
	r.Equal(1, mempool.Len(ctx))
	r.True(mempool.Has(ctx, tx0.GetID()))
}

func TestAccepted(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	network := NewVMTestNetwork(ctx, t, 2)
	network.SetState(ctx, avasnow.NormalOp)
	defer network.Shutdown(ctx)

	client := jsonrpc.NewJSONRPCClient(network.URIs()[0])
	blockID, blockHeight, timestamp, err := client.Accepted(ctx)
	r.NoError(err)
	genesisBlock, err := network.VMs[0].SnowVM.GetConsensusIndex().GetLastAccepted(ctx)
	r.NoError(err)
	r.Equal(genesisBlock.GetID(), blockID)
	r.Equal(uint64(0), blockHeight)
	r.Equal(genesisBlock.GetTimestamp(), timestamp)

	tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.AuthFactories()[0])
	r.NoError(err)
	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx}))

	blockID, blockHeight, timestamp, err = client.Accepted(ctx)
	r.NoError(err)
	blk1, err := network.VMs[0].SnowVM.GetConsensusIndex().GetLastAccepted(ctx)
	r.NoError(err)
	r.Equal(blk1.GetID(), blockID)
	r.Equal(uint64(1), blockHeight)
	r.Equal(blk1.GetTimestamp(), timestamp)
}

func TestExternalSubscriber(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)

	throwawayNetwork := NewVMTestNetwork(ctx, t, 1)
	createParserFromBytes := func(_ []byte) (chain.Parser, error) {
		return throwawayNetwork.Configuration().Parser(), nil
	}

	listener, err := grpcutils.NewListener()
	r.NoError(err)
	serverCloser := grpcutils.ServerCloser{}

	externalSubscriberAcceptedBlocksCh := make(chan ids.ID, 1)
	externalSubscriber0 := externalsubscriber.NewExternalSubscriberServer(logging.NoLog{}, createParserFromBytes, []event.Subscription[*chain.ExecutedBlock]{
		event.SubscriptionFunc[*chain.ExecutedBlock]{
			NotifyF: func(_ context.Context, blk *chain.ExecutedBlock) error {
				externalSubscriberAcceptedBlocksCh <- blk.Block.GetID()
				return nil
			},
		},
	})

	server := grpcutils.NewServer()
	pb.RegisterExternalSubscriberServer(server, externalSubscriber0)
	serverCloser.Add(server)

	go grpcutils.Serve(listener, server)

	t.Cleanup(func() {
		serverCloser.Stop()
		_ = listener.Close()
	})

	config := hcontext.NewEmptyConfig()
	r.NoError(hcontext.SetConfig(config, externalsubscriber.Namespace, externalsubscriber.Config{
		Enabled:       true,
		ServerAddress: listener.Addr().String(),
	}))

	configBytes, err := json.Marshal(config)
	r.NoError(err)

	network := NewVMTestNetwork(ctx, t, 1, WithConfigBytes(func() []byte { return configBytes }))
	network.SetState(ctx, avasnow.NormalOp)
	defer network.Shutdown(ctx)

	// Confirm and throw away last accepted block on startup
	genesisBlkID := <-externalSubscriberAcceptedBlocksCh
	lastAccepted, err := network.VMs[0].SnowVM.GetConsensusIndex().GetLastAccepted(ctx)
	r.NoError(err)
	r.Equal(lastAccepted.GetID(), genesisBlkID)

	tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.AuthFactories()[0])
	r.NoError(err)
	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx}))

	acceptedBlkID := <-externalSubscriberAcceptedBlocksCh
	lastAccepted, err = network.VMs[0].SnowVM.GetConsensusIndex().GetLastAccepted(ctx)
	r.NoError(err)
	r.Equal(lastAccepted.GetID(), acceptedBlkID)
}

func TestDirectStateAPI(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	network := NewVMTestNetwork(ctx, t, 2)
	network.SetState(ctx, avasnow.NormalOp)
	defer network.Shutdown(ctx)

	client := stateapi.NewJSONRPCStateClient(network.URIs()[0])

	key := keys.EncodeChunks([]byte{1, 2, 3}, 1)
	value := []byte{4, 5, 6}
	vals, errs, err := client.ReadState(ctx, [][]byte{key})
	r.NoError(err)
	r.Len(vals, 1)
	r.Len(errs, 1)
	r.Nil(vals[0])
	// Cannot use ErrorIs because the error is returned over the API
	r.ErrorContains(errs[0], database.ErrNotFound.Error())

	keys := [][]byte{key}
	values := [][]byte{value}
	stateKeys := state.Keys{string(key): state.All}
	tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits:    1,
		SpecifiedStateKeys: stateKeys,
		WriteKeys:          keys,
		WriteValues:        values,
	}}, network.AuthFactories()[0])
	r.NoError(err)
	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx}))

	vals, errs, err = client.ReadState(ctx, [][]byte{key})
	r.NoError(err)
	r.Len(vals, 1)
	r.Len(errs, 1)
	r.NoError(errs[0])
	r.Equal(value, vals[0])
}

func TestIndexerAPI(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	network := NewVMTestNetwork(ctx, t, 2)
	network.SetState(ctx, avasnow.NormalOp)
	defer network.Shutdown(ctx)

	client := indexer.NewClient(network.URIs()[0])
	parser := network.VMs[0].VM
	genesisBlock, err := client.GetLatestBlock(ctx, parser)
	r.NoError(err)
	lastAccepted, err := network.VMs[0].SnowVM.GetConsensusIndex().GetLastAccepted(ctx)
	r.NoError(err)
	r.Equal(lastAccepted.GetID(), genesisBlock.Block.GetID())
	r.Equal(uint64(0), genesisBlock.Block.GetHeight())

	tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.AuthFactories()[0])
	r.NoError(err)
	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx}))

	blk1, err := client.GetLatestBlock(ctx, parser)
	r.NoError(err)
	lastAccepted, err = network.VMs[0].SnowVM.GetConsensusIndex().GetLastAccepted(ctx)
	r.NoError(err)
	r.Equal(lastAccepted.GetID(), blk1.Block.GetID())
	r.Equal(uint64(1), blk1.Block.GetHeight())

	txRes, success, err := client.GetTx(ctx, tx.GetID())
	r.NoError(err)
	r.True(success)
	r.True(txRes.Success)
}

func TestWebsocketAPI(t *testing.T) {
	// XXX: re-write or remove ws server. RegisterTx is async, so we SHOULD
	// need to run a goroutine in this test to either confirm the block or
	// confirm receipt of the tx/block on the ws client. However, implementing it
	// this way does not cause the test to pass, whereas adding this sleep does.
	// Including the integration test until we remove or re-write the ws client/server.
	t.Skip()
	ctx := context.Background()
	r := require.New(t)
	network := NewVMTestNetwork(ctx, t, 1)
	network.SetState(ctx, avasnow.NormalOp)
	defer network.Shutdown(ctx)

	client, err := ws.NewWebSocketClient(network.URIs()[0], time.Second, 10, consts.NetworkSizeLimit)
	defer func() {
		r.NoError(client.Close())
	}()
	r.NoError(err)

	r.NoError(client.RegisterBlocks())

	tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.AuthFactories()[0])
	r.NoError(err)
	r.NoError(client.RegisterTx(tx))

	time.Sleep(time.Second)
	blks := network.BuildBlockAndUpdateHead(ctx)
	for i, blk := range blks {
		r.NoError(blk.Accept(ctx), "failed to accept block at VM index %d", i)
	}

	wsBlk, wsResults, wsUnitPrices, err := client.ListenBlock(ctx, network.VMs[0].VM)
	r.NoError(err)

	txID, txErr, res, unpackErr := client.ListenTx(ctx)
	r.NoError(unpackErr)
	r.NoError(txErr)
	r.Equal(tx.GetID(), txID)
	r.True(res.Success)

	lastAccepted, err := network.VMs[0].SnowVM.GetConsensusIndex().GetLastAccepted(ctx)
	r.NoError(err)
	r.Equal(lastAccepted.GetID(), wsBlk.GetID())
	r.Equal(lastAccepted.ExecutionResults.Results, wsResults)
	r.Equal(lastAccepted.ExecutionResults.UnitPrices, wsUnitPrices)
}

func TestSkipStateSync(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	network := NewVMTestNetwork(ctx, t, 1)

	initialVM := network.VMs[0]
	initialVMGenesisBlock, err := initialVM.SnowVM.GetConsensusIndex().GetLastAccepted(ctx)
	r.NoError(err)

	numBlocks := 5
	network.ConfirmBlocks(ctx, numBlocks, func(i int) []*chain.Transaction {
		tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
			NumComputeUnits: 1,
			Nonce:           uint64(i),
			SpecifiedStateKeys: state.Keys{
				string(keys.EncodeChunks([]byte{byte(i)}, 1)): state.All,
			},
			WriteKeys:   [][]byte{keys.EncodeChunks([]byte{byte(i)}, 1)},
			WriteValues: [][]byte{{byte(i)}},
		}}, network.AuthFactories()[0])
		r.NoError(err)
		return []*chain.Transaction{tx}
	})

	config := map[string]interface{}{
		vm.StateSyncNamespace: map[string]interface{}{
			"minBlocks": uint64(numBlocks + 1),
		},
	}
	configBytes, err := json.Marshal(config)
	r.NoError(err)

	vm := network.AddVM(ctx, configBytes)
	lastAccepted, err := vm.SnowVM.GetConsensusIndex().GetLastAccepted(ctx)
	r.NoError(err)
	r.Equal(lastAccepted.GetID(), initialVMGenesisBlock.GetID())

	stateSummary, err := initialVM.SnowVM.GetLastStateSummary(ctx)
	r.NoError(err)
	parsedStateSummary, err := vm.SnowVM.ParseStateSummary(ctx, stateSummary.Bytes())
	r.NoError(err)

	// Accepting the state sync summary returns Skipped and does not change
	// the last accepted block.
	stateSyncMode, err := parsedStateSummary.Accept(ctx)
	r.NoError(err)
	r.Equal(block.StateSyncSkipped, stateSyncMode)
	r.Equal(lastAccepted.GetID(), initialVMGenesisBlock.GetID())

	r.NoError(vm.SnowVM.SetState(ctx, avasnow.Bootstrapping))
	r.NoError(vm.SnowVM.SetState(ctx, avasnow.NormalOp))
	_, err = vm.SnowVM.HealthCheck(ctx)
	r.NoError(err)

	network.SynchronizeNetwork(ctx)
	blk, err := vm.SnowVM.GetConsensusIndex().GetLastAccepted(ctx)
	r.NoError(err)
	r.Equal(blk.GetHeight(), uint64(numBlocks))
}

func TestStateSync(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	network := NewVMTestNetwork(ctx, t, 1, WithGenesisRulesOverride(func(rules *genesis.Rules) {
		rules.ValidityWindow = time.Second.Milliseconds()
	}))

	initialVM := network.VMs[0]
	initialVMGenesisBlock, err := initialVM.SnowVM.GetConsensusIndex().GetLastAccepted(ctx)
	r.NoError(err)

	numBlocks := 5
	nonce := uint64(0)
	network.ConfirmBlocks(ctx, numBlocks, func(i int) []*chain.Transaction {
		nonce++
		tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
			NumComputeUnits: 1,
			Nonce:           nonce,
			SpecifiedStateKeys: state.Keys{
				string(keys.EncodeChunks([]byte{byte(i)}, 1)): state.All,
			},
			WriteKeys:   [][]byte{keys.EncodeChunks([]byte{byte(i)}, 1)},
			WriteValues: [][]byte{{byte(i)}},
		}}, network.AuthFactories()[0])
		r.NoError(err)
		return []*chain.Transaction{tx}
	})

	config := map[string]interface{}{
		vm.StateSyncNamespace: map[string]interface{}{
			"minBlocks": uint64(numBlocks - 1),
		},
	}
	configBytes, err := json.Marshal(config)
	r.NoError(err)

	vm := network.AddVM(ctx, configBytes)
	lastAccepted, err := vm.SnowVM.GetConsensusIndex().GetLastAccepted(ctx)
	r.NoError(err)
	r.Equal(lastAccepted.GetID(), initialVMGenesisBlock.GetID())

	stateSummary, err := initialVM.SnowVM.GetLastStateSummary(ctx)
	r.NoError(err)
	parsedStateSummary, err := vm.SnowVM.ParseStateSummary(ctx, stateSummary.Bytes())
	r.NoError(err)

	// Accepting the state sync summary kicks off an async process
	stateSyncMode, err := parsedStateSummary.Accept(ctx)
	r.NoError(err)
	r.Equal(block.StateSyncDynamic, stateSyncMode)

	// Keep producing blocks until sync finishes
	// Note: could also stop producing blocks once we have produced a validity window of blocks
	// after sync started.
	stopCh := make(chan struct{})
	stoppedCh := make(chan struct{})
	go func() {
		defer close(stoppedCh)
		nonce := numBlocks + 1
		for {
			network.ConfirmBlocks(ctx, numBlocks, func(i int) []*chain.Transaction {
				nonce++
				tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
					NumComputeUnits: 1,
					Nonce:           uint64(nonce),
					SpecifiedStateKeys: state.Keys{
						string(keys.EncodeChunks([]byte{byte(i)}, 1)): state.All,
					},
					WriteKeys:   [][]byte{keys.EncodeChunks([]byte{byte(i)}, 1)},
					WriteValues: [][]byte{{byte(i)}},
				}}, network.AuthFactories()[0])
				r.NoError(err)
				return []*chain.Transaction{tx}
			})
			select {
			case <-stopCh:
				return
			default:
			}
		}
	}()

	awaitSyncCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	err = vm.VM.SyncClient.Wait(awaitSyncCtx)
	close(stopCh)
	r.NoError(err)
	_, err = vm.SnowVM.HealthCheck(ctx)
	r.NoError(err)
	<-stoppedCh

	network.ConfirmBlocks(ctx, 1, func(int) []*chain.Transaction {
		nonce++
		tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
			NumComputeUnits: 1,
			Nonce:           nonce,
		}}, network.AuthFactories()[0])
		r.NoError(err)
		return []*chain.Transaction{tx}
	})
}

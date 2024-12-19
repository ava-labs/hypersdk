// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	avasnow "github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/enginetest"
	"github.com/ava-labs/avalanchego/snow/snowtest"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/grpcutils"
	"github.com/ava-labs/avalanchego/x/merkledb"
	"github.com/ava-labs/hypersdk/api/indexer"
	"github.com/ava-labs/hypersdk/api/jsonrpc"
	stateapi "github.com/ava-labs/hypersdk/api/state"
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
	"github.com/ava-labs/hypersdk/keys"
	pb "github.com/ava-labs/hypersdk/proto/pb/externalsubscriber"
	"github.com/ava-labs/hypersdk/snow"
	"github.com/ava-labs/hypersdk/state"
	"github.com/ava-labs/hypersdk/state/balance"
	"github.com/ava-labs/hypersdk/state/metadata"
	"github.com/ava-labs/hypersdk/tests/workload"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/vm"
	"github.com/ava-labs/hypersdk/vm/defaultvm"
	"github.com/stretchr/testify/require"
)

var _ workload.TestNetwork = (*TestNetwork)(nil)

type VM struct {
	nodeID    ids.NodeID
	appSender *enginetest.Sender

	SnowVM *snow.VM[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]
	VM     *vm.VM

	toEngine chan common.Message
	server   *httptest.Server
}

func NewTestVM(
	ctx context.Context,
	t *testing.T,
	chainID ids.ID,
	configBytes []byte,
	allocations []*genesis.CustomAllocation,
) *VM {
	r := require.New(t)
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
	options := defaultvm.NewDefaultOptions()
	options = append(options, vm.WithManual())
	vm, err := vm.New(
		genesis.DefaultGenesisFactory{},
		balance.NewPrefixBalanceHandler([]byte{0}),
		metadata.NewDefaultManager(),
		actionParser,
		authParser,
		outputParser,
		auth.Engines(),
		options...,
	)
	r.NoError(err)
	r.NotNil(vm)
	snowVM := snow.NewVM(vm)

	toEngine := make(chan common.Message, 1)
	testRules := genesis.NewDefaultRules()
	testRules.MinBlockGap = 0
	testRules.MinEmptyBlockGap = 0
	genesis := genesis.DefaultGenesis{
		StateBranchFactor: merkledb.BranchFactor16,
		CustomAllocation:  allocations,
		Rules:             testRules,
	}
	genesisBytes, err := json.Marshal(genesis)
	r.NoError(err)
	snowCtx := snowtest.Context(t, chainID)
	snowCtx.ChainDataDir = t.TempDir()
	appSender := &enginetest.Sender{T: t}
	r.NoError(snowVM.Initialize(ctx, snowCtx, nil, genesisBytes, nil, configBytes, toEngine, nil, appSender))

	router := http.NewServeMux()
	handlers, err := snowVM.CreateHandlers(ctx)
	r.NoError(err)
	for endpoint, handler := range handlers {
		router.Handle(endpoint, handler)
	}
	server := httptest.NewServer(router)
	return &VM{
		nodeID:    snowCtx.NodeID,
		appSender: appSender,
		SnowVM:    snowVM,
		VM:        vm,
		toEngine:  toEngine,
		server:    server,
	}
}

type TestNetwork struct {
	chainID    ids.ID
	require    *require.Assertions
	VMs        []*VM
	nodeIDToVM map[ids.NodeID]*VM

	authFactory   chain.AuthFactory
	uris          []string
	configuration workload.TestNetworkConfiguration
}

func NewTestNetwork(
	ctx context.Context,
	t *testing.T,
	chainID ids.ID,
	numVMs int,
	configBytes []byte,
) *TestNetwork {
	r := require.New(t)
	privKey, err := ed25519.GeneratePrivateKey()
	r.NoError(err)
	authFactory := auth.NewED25519Factory(privKey)
	funds := uint64(1_000_000_000)
	vms := make([]*VM, numVMs)
	allocations := []*genesis.CustomAllocation{
		{
			Address: authFactory.Address(),
			Balance: funds,
		},
	}
	nodeIDToVM := make(map[ids.NodeID]*VM)
	uris := make([]string, len(vms))
	for i := range vms {
		vm := NewTestVM(ctx, t, chainID, configBytes, allocations)
		vms[i] = vm
		uris[i] = vm.server.URL
		nodeIDToVM[vm.nodeID] = vm
	}
	configuration := workload.NewDefaultTestNetworkConfiguration(
		vms[0].VM.GenesisBytes,
		"hypervmtests",
		vms[0].VM,
		[]chain.AuthFactory{authFactory},
	)
	testNetwork := &TestNetwork{
		chainID:       chainID,
		require:       r,
		VMs:           vms,
		authFactory:   authFactory,
		uris:          uris,
		configuration: configuration,
		nodeIDToVM:    nodeIDToVM,
	}
	testNetwork.initAppNetwork()
	return testNetwork
}

func (n *TestNetwork) initAppNetwork() {
	for _, vm := range n.VMs {
		myNodeID := vm.nodeID
		vm.appSender.SendAppRequestF = func(ctx context.Context, nodeIDs set.Set[ids.NodeID], u uint32, b []byte) error {
			for nodeID := range nodeIDs {
				if nodeID == myNodeID {
					go func() {
						err := vm.SnowVM.AppRequest(ctx, nodeID, u, time.Now().Add(time.Second), b)
						n.require.NoError(err)
					}()
				} else {
					err := n.nodeIDToVM[nodeID].SnowVM.AppRequest(ctx, nodeID, u, time.Now().Add(time.Second), b)
					n.require.NoError(err)
				}
			}
			return nil
		}
		vm.appSender.SendAppResponseF = func(ctx context.Context, nodeID ids.NodeID, requestID uint32, response []byte) error {
			if nodeID == myNodeID {
				go func() {
					err := vm.SnowVM.AppResponse(ctx, nodeID, requestID, response)
					n.require.NoError(err)
				}()
				return nil
			}

			return n.nodeIDToVM[nodeID].SnowVM.AppResponse(ctx, nodeID, requestID, response)
		}
		vm.appSender.SendAppErrorF = func(ctx context.Context, nodeID ids.NodeID, requestID uint32, code int32, message string) error {
			if nodeID == myNodeID {
				go func() {
					err := vm.SnowVM.AppRequestFailed(ctx, nodeID, requestID, &common.AppError{
						Code:    code,
						Message: message,
					})
					n.require.NoError(err)
				}()
				return nil
			}
			return n.nodeIDToVM[nodeID].SnowVM.AppRequestFailed(ctx, nodeID, requestID, &common.AppError{
				Code:    code,
				Message: message,
			})
		}
		vm.appSender.SendAppGossipF = func(ctx context.Context, sendConfig common.SendConfig, b []byte) error {
			nodeIDs := sendConfig.NodeIDs
			nodeIDs.Remove(myNodeID)
			// Select numSend nodes excluding myNodeID and gossip to them
			numSend := sendConfig.Validators + sendConfig.NonValidators + sendConfig.Peers
			nodes := set.NewSet[ids.NodeID](numSend)
			for nodeID := range n.nodeIDToVM {
				if nodeID == myNodeID {
					continue
				}
				nodes.Add(nodeID)
				if nodes.Len() >= numSend {
					break
				}
			}

			// Send to specified nodes
			for nodeID := range nodeIDs {
				err := n.nodeIDToVM[nodeID].SnowVM.AppGossip(ctx, nodeID, b)
				n.require.NoError(err)
			}
			return nil
		}
	}
}

func (n *TestNetwork) SetState(ctx context.Context, state avasnow.State) {
	for _, vm := range n.VMs {
		n.require.NoError(vm.SnowVM.SetState(ctx, state))
	}
}

// ConfirmInvalidTx confirms that attempting to issue the transaction to the mempool results in the target error
func (n *TestNetwork) ConfirmInvalidTx(ctx context.Context, tx *chain.Transaction, targetErr error) {
	err := n.VMs[0].VM.SubmitTx(ctx, tx)
	n.require.ErrorIs(err, targetErr)

	// TODO: manually construct a block and confirm that attempting to execute the block against tip results in the same
	// target error.
	// This requires a refactor of block building to easily construct a block while skipping over tx validity checks.
}

func (n *TestNetwork) SubmitTxs(ctx context.Context, txs []*chain.Transaction) {
	// Submit tx to first node
	err := errors.Join(n.VMs[0].VM.Submit(ctx, txs)...)
	n.require.NoError(err)
}

func (n *TestNetwork) BuildBlockAndUpdateHead(ctx context.Context) []*snow.StatefulBlock[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock] {
	n.require.NoError(n.VMs[0].VM.Builder().Force(ctx))
	select {
	case <-n.VMs[0].toEngine:
	case <-time.After(time.Second):
		n.require.Fail("timeout waiting for PendingTxs message")
	}

	blk, err := n.VMs[0].SnowVM.GetCovariantVM().BuildBlock(ctx)
	n.require.NoError(err)

	n.require.NoError(blk.Verify(ctx))
	n.require.NoError(n.VMs[0].SnowVM.SetPreference(ctx, blk.ID()))

	blks := make([]*snow.StatefulBlock[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock], len(n.VMs))
	blks[0] = blk
	for i, otherVM := range n.VMs[1:] {
		parsedBlk, err := otherVM.SnowVM.GetCovariantVM().ParseBlock(ctx, blk.Bytes())
		n.require.NoError(err)
		n.require.Equal(blk.ID(), parsedBlk.ID())

		n.require.NoError(parsedBlk.Verify(ctx))
		n.require.NoError(otherVM.SnowVM.SetPreference(ctx, blk.ID()))
		blks[i+1] = parsedBlk
	}

	return blks
}

func (n *TestNetwork) ConfirmTxsInBlock(ctx context.Context, blk *chain.ExecutionBlock, txs []*chain.Transaction) {
	for _, tx := range txs {
		n.require.True(blk.ContainsTx(tx.GetID()))
	}
}

func (n *TestNetwork) ConfirmTxs(ctx context.Context, txs []*chain.Transaction) error {
	n.SubmitTxs(ctx, txs)
	blks := n.BuildBlockAndUpdateHead(ctx)

	outputBlock := blks[0].Output
	n.require.NotNil(outputBlock)

	n.ConfirmTxsInBlock(ctx, outputBlock.ExecutionBlock, txs)

	for i, blk := range blks {
		n.require.NoError(blk.Accept(ctx), "failed to accept block at VM index %d", i)
	}
	return nil
}

func (n *TestNetwork) GenerateTx(ctx context.Context, actions []chain.Action, authFactory chain.AuthFactory) (*chain.Transaction, error) {
	cli := jsonrpc.NewJSONRPCClient(n.VMs[0].server.URL)
	_, tx, _, err := cli.GenerateTransaction(ctx, n.VMs[0].VM, actions, authFactory)
	return tx, err
}

func (n *TestNetwork) URIs() []string {
	return n.uris
}

func (n *TestNetwork) Configuration() workload.TestNetworkConfiguration {
	return n.configuration
}

func TestEmptyBlock(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	chainID := ids.GenerateTestID()
	network := NewTestNetwork(ctx, t, chainID, 2, nil)

	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{}))
}

func TestValidBlocks(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	chainID := ids.GenerateTestID()
	network := NewTestNetwork(ctx, t, chainID, 2, nil)

	tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.authFactory)
	r.NoError(err)
	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx}))

	tx, err = network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
		Nonce:           1,
	}}, network.authFactory)
	r.NoError(err)
	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx}))
}

func TestSubmitTx(t *testing.T) {
	tests := []struct {
		name      string
		makeTx    func(r *require.Assertions, network *TestNetwork) *chain.Transaction
		targetErr error
	}{
		{
			name: "valid tx",
			makeTx: func(r *require.Assertions, network *TestNetwork) *chain.Transaction {
				unsignedTx := chain.NewTxData(
					&chain.Base{
						ChainID:   network.chainID,
						Timestamp: utils.UnixRMilli(time.Now().UnixMilli(), 30_000),
						MaxFee:    1_000,
					},
					[]chain.Action{&chaintest.TestAction{NumComputeUnits: 1}},
				)
				tx, err := unsignedTx.Sign(network.authFactory)
				r.NoError(err)
				return tx
			},
			targetErr: nil,
		},
		{
			name: chain.ErrMisalignedTime.Error(),
			makeTx: func(r *require.Assertions, network *TestNetwork) *chain.Transaction {
				unsignedTx := chain.NewTxData(
					&chain.Base{
						ChainID:   network.chainID,
						Timestamp: 1,
						MaxFee:    1_000,
					},
					[]chain.Action{&chaintest.TestAction{NumComputeUnits: 1}},
				)
				tx, err := unsignedTx.Sign(network.authFactory)
				r.NoError(err)
				return tx
			},
			targetErr: chain.ErrMisalignedTime,
		},
		{
			name: chain.ErrTimestampTooLate.Error(),
			makeTx: func(r *require.Assertions, network *TestNetwork) *chain.Transaction {
				unsignedTx := chain.NewTxData(
					&chain.Base{
						ChainID:   network.chainID,
						Timestamp: int64(time.Millisecond),
						MaxFee:    1_000,
					},
					[]chain.Action{&chaintest.TestAction{NumComputeUnits: 1}},
				)
				tx, err := unsignedTx.Sign(network.authFactory)
				r.NoError(err)
				return tx
			},
			targetErr: chain.ErrTimestampTooLate,
		},
		{
			name: chain.ErrTimestampTooEarly.Error(),
			makeTx: func(r *require.Assertions, network *TestNetwork) *chain.Transaction {
				unsignedTx := chain.NewTxData(
					&chain.Base{
						ChainID:   network.chainID,
						Timestamp: utils.UnixRMilli(time.Now().UnixMilli(), time.Hour.Milliseconds()),
						MaxFee:    1_000,
					},
					[]chain.Action{&chaintest.TestAction{NumComputeUnits: 1}},
				)
				tx, err := unsignedTx.Sign(network.authFactory)
				r.NoError(err)
				return tx
			},
			targetErr: chain.ErrTimestampTooEarly,
		},
		{
			name: "invalid auth",
			makeTx: func(r *require.Assertions, network *TestNetwork) *chain.Transaction {
				unsignedTx := chain.NewTxData(
					&chain.Base{
						ChainID:   network.chainID,
						Timestamp: utils.UnixRMilli(time.Now().UnixMilli(), 30_000),
						MaxFee:    1_000,
					},
					[]chain.Action{&chaintest.TestAction{NumComputeUnits: 1}},
				)
				invalidAuth, err := network.authFactory.Sign([]byte{0})
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
			chainID := ids.GenerateTestID()
			network := NewTestNetwork(ctx, t, chainID, 2, nil)

			invalidTx := test.makeTx(r, network)
			network.ConfirmInvalidTx(ctx, invalidTx, test.targetErr)
		})
	}
}

func TestValidityWindowDuplicateAcceptedBlock(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	chainID := ids.GenerateTestID()
	network := NewTestNetwork(ctx, t, chainID, 2, nil)

	tx0, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.authFactory)
	r.NoError(err)

	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx0}))

	network.ConfirmInvalidTx(ctx, tx0, chain.ErrDuplicateTx)

	// Build another block, so that the duplicate is in an accepted ancestor
	// instead of the direct parent (last accepted block)
	tx1, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
		Nonce:           1,
	}}, network.authFactory)
	r.NoError(err)
	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx1}))

	// The duplicate transaction should still fail
	network.ConfirmInvalidTx(ctx, tx0, chain.ErrDuplicateTx)
}

func TestValidityWindowDuplicateProcessingAncestor(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	chainID := ids.GenerateTestID()
	network := NewTestNetwork(ctx, t, chainID, 2, nil)

	tx0, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.authFactory)
	r.NoError(err)
	tx1, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
		Nonce:           1,
	}}, network.authFactory)
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
	network.ConfirmTxsInBlock(ctx, vmBlk2s[0].Output.ExecutionBlock, txs0)

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
	chainID := ids.GenerateTestID()
	network := NewTestNetwork(ctx, t, chainID, 2, nil)

	tx0, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.authFactory)
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
	chainID := ids.GenerateTestID()
	network := NewTestNetwork(ctx, t, chainID, 2, nil)
	network.SetState(ctx, avasnow.NormalOp)

	tx0, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.authFactory)
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
	chainID := ids.GenerateTestID()
	network := NewTestNetwork(ctx, t, chainID, 2, nil)

	client := jsonrpc.NewJSONRPCClient(network.VMs[0].server.URL)
	blockID, blockHeight, timestamp, err := client.Accepted(ctx)
	r.NoError(err)
	genesisBlock := network.VMs[0].SnowVM.GetCovariantVM().LastAcceptedBlock(ctx)
	r.NoError(err)
	r.Equal(genesisBlock.ID(), blockID)
	r.Equal(uint64(0), blockHeight)
	r.Equal(genesisBlock.Timestamp().UnixMilli(), timestamp)

	tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.authFactory)
	r.NoError(err)
	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx}))

	blockID, blockHeight, timestamp, err = client.Accepted(ctx)
	r.NoError(err)
	blk1 := network.VMs[0].SnowVM.GetCovariantVM().LastAcceptedBlock(ctx)
	r.NoError(err)
	r.Equal(blk1.ID(), blockID)
	r.Equal(uint64(1), blockHeight)
	r.Equal(blk1.Timestamp().UnixMilli(), timestamp)
}

func TestExternalSubscriber(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	chainID := ids.GenerateTestID()

	throwawayNetwork := NewTestNetwork(ctx, t, chainID, 1, nil)
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
				externalSubscriberAcceptedBlocksCh <- blk.Block.ID()
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

	subscriberConfig := externalsubscriber.Config{
		Enabled:       true,
		ServerAddress: listener.Addr().String(),
	}
	subscriberConfigBytes, err := json.Marshal(subscriberConfig)
	r.NoError(err)
	vmConfig := vm.NewConfig()
	namespacedConfig := map[string]json.RawMessage{
		externalsubscriber.Namespace: subscriberConfigBytes,
	}
	vmConfig.ServiceConfig = namespacedConfig
	configBytes, err := json.Marshal(vmConfig)
	r.NoError(err)

	network := NewTestNetwork(ctx, t, chainID, 1, configBytes)

	tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.authFactory)
	r.NoError(err)
	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx}))

	var acceptedBlkID ids.ID
	select {
	case acceptedBlkID = <-externalSubscriberAcceptedBlocksCh:
	case <-time.After(time.Second):
		r.Fail("timeout waiting for external subscriber to receive accepted block")
	}
	r.Equal(network.VMs[0].SnowVM.GetCovariantVM().LastAcceptedBlock(ctx).ID(), acceptedBlkID)
}

func TestDirectStateAPI(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	chainID := ids.GenerateTestID()
	network := NewTestNetwork(ctx, t, chainID, 2, nil)

	client := stateapi.NewJSONRPCStateClient(network.VMs[0].server.URL)

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
	}}, network.authFactory)
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
	chainID := ids.GenerateTestID()
	network := NewTestNetwork(ctx, t, chainID, 2, nil)

	client := indexer.NewClient(network.VMs[0].server.URL)
	parser := network.VMs[0].VM
	genesisBlock, err := client.GetLatestBlock(ctx, parser)
	r.NoError(err)
	lastAccepted := network.VMs[0].SnowVM.GetChainIndex().GetLastAccepted(ctx)
	r.Equal(lastAccepted.ID(), genesisBlock.Block.ID())
	r.Equal(uint64(0), genesisBlock.Block.Height())

	tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.authFactory)
	r.NoError(err)
	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx}))

	blk1, err := client.GetLatestBlock(ctx, parser)
	r.NoError(err)
	lastAccepted = network.VMs[0].SnowVM.GetChainIndex().GetLastAccepted(ctx)
	r.Equal(lastAccepted.ID(), blk1.Block.ID())
	r.Equal(uint64(1), blk1.Block.Height())

	txRes, success, err := client.GetTx(ctx, tx.GetID())
	r.NoError(err)
	r.True(success)
	r.True(txRes.Success)
}

func TestWebsocketAPI(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	chainID := ids.GenerateTestID()
	network := NewTestNetwork(ctx, t, chainID, 1, nil)

	client, err := ws.NewWebSocketClient(network.VMs[0].server.URL, time.Second, 10, consts.NetworkSizeLimit)
	r.NoError(err)

	r.NoError(client.RegisterBlocks())

	tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.authFactory)
	r.NoError(err)
	r.NoError(client.RegisterTx(tx))

	// Removing this sleep causes the test to fail because RegisterTx is somehow async...
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

	lastAccepted := network.VMs[0].SnowVM.GetChainIndex().GetLastAccepted(ctx)
	r.Equal(lastAccepted.ID(), wsBlk.ID())
	r.Equal(lastAccepted.ExecutionResults.Results, wsResults)
	r.Equal(lastAccepted.ExecutionResults.UnitPrices, wsUnitPrices)
}

// APIs
// state sync
// add back async acceptor and send notification of genesis on startup

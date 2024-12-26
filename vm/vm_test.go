// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	avasnow "github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/enginetest"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/snow/snowtest"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/version"
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
	require *require.Assertions

	snowCtx   *avasnow.Context
	nodeID    ids.NodeID
	appSender *enginetest.Sender
	toEngine  chan common.Message

	SnowVM *snow.VM[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock]
	VM     *vm.VM

	server *httptest.Server
}

func NewTestVM(
	ctx context.Context,
	t *testing.T,
	chainID ids.ID,
	genesisBytes []byte,
	configBytes []byte,
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

	snowCtx := snowtest.Context(t, chainID)
	var logCores []logging.WrappedCore
	if os.Getenv("HYPERVM_TEST_LOGGING") == "true" {
		logCores = append(logCores, logging.NewWrappedCore(
			logging.Debug,
			os.Stdout,
			logging.Colors.ConsoleEncoder(),
		))
	}
	log := logging.NewLogger(
		"vmtest",
		logCores...,
	)

	snowCtx.Log = log
	snowCtx.ChainDataDir = t.TempDir()
	snowCtx.NodeID = ids.GenerateTestNodeID()
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
		require:   r,
		nodeID:    snowCtx.NodeID,
		appSender: appSender,
		SnowVM:    snowVM,
		VM:        vm,
		snowCtx:   snowCtx,
		toEngine:  toEngine,
		server:    server,
	}
}

func (vm *VM) BuildAndSetPreference(ctx context.Context) *snow.StatefulBlock[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock] {
	vm.snowCtx.Lock.Lock()
	defer vm.snowCtx.Lock.Unlock()

	blk, err := vm.SnowVM.GetCovariantVM().BuildBlock(ctx)
	vm.require.NoError(err)

	vm.require.NoError(blk.Verify(ctx))
	vm.require.NoError(vm.SnowVM.SetPreference(ctx, blk.ID()))
	return blk
}

func (vm *VM) ParseAndSetPreference(ctx context.Context, bytes []byte) *snow.StatefulBlock[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock] {
	vm.snowCtx.Lock.Lock()
	defer vm.snowCtx.Lock.Unlock()

	blk, err := vm.SnowVM.GetCovariantVM().ParseBlock(ctx, bytes)
	vm.require.NoError(err)

	vm.require.NoError(blk.Verify(ctx))
	vm.require.NoError(vm.SnowVM.SetPreference(ctx, blk.ID()))
	return blk
}

type TestNetwork struct {
	t       *testing.T
	require *require.Assertions

	chainID      ids.ID
	genesisBytes []byte
	authFactory  chain.AuthFactory

	VMs        []*VM
	nodeIDToVM map[ids.NodeID]*VM
	uris       []string

	configuration workload.TestNetworkConfiguration
}

type NetworkOptions struct {
	GenesisRulesOverride func(rules *genesis.Rules)
}

type NetworkOption func(*NetworkOptions)

func WithRulesOverride(override func(rules *genesis.Rules)) NetworkOption {
	return func(opts *NetworkOptions) {
		opts.GenesisRulesOverride = override
	}
}

func NewNetworkOptions(opts ...NetworkOption) *NetworkOptions {
	options := &NetworkOptions{}
	for _, opt := range opts {
		opt(options)
	}
	return options
}

func NewTestNetwork(
	ctx context.Context,
	t *testing.T,
	chainID ids.ID,
	numVMs int,
	configBytes []byte,
	opts ...NetworkOption,
) *TestNetwork {
	r := require.New(t)
	networkOptions := NewNetworkOptions(opts...)
	privKey, err := ed25519.GeneratePrivateKey()
	r.NoError(err)
	authFactory := auth.NewED25519Factory(privKey)
	funds := uint64(1_000_000_000)
	allocations := []*genesis.CustomAllocation{
		{
			Address: authFactory.Address(),
			Balance: funds,
		},
	}
	testRules := genesis.NewDefaultRules()
	testRules.MinBlockGap = 0
	testRules.MinEmptyBlockGap = 0
	if networkOptions.GenesisRulesOverride != nil {
		networkOptions.GenesisRulesOverride(testRules)
	}
	genesis := genesis.DefaultGenesis{
		StateBranchFactor: merkledb.BranchFactor16,
		CustomAllocation:  allocations,
		Rules:             testRules,
	}
	genesisBytes, err := json.Marshal(genesis)
	r.NoError(err)

	testNetwork := &TestNetwork{
		t:            t,
		require:      r,
		chainID:      chainID,
		genesisBytes: genesisBytes,
		authFactory:  authFactory,
	}

	vms := make([]*VM, numVMs)
	nodeIDToVM := make(map[ids.NodeID]*VM)
	uris := make([]string, len(vms))
	for i := range vms {
		vm := NewTestVM(ctx, t, chainID, genesisBytes, configBytes)
		vms[i] = vm
		uris[i] = vm.server.URL
		nodeIDToVM[vm.nodeID] = vm
	}
	testNetwork.VMs = vms
	testNetwork.nodeIDToVM = nodeIDToVM
	testNetwork.uris = uris
	configuration := workload.NewDefaultTestNetworkConfiguration(
		vms[0].VM.GenesisBytes,
		"hypervmtests",
		vms[0].VM,
		[]chain.AuthFactory{authFactory},
	)
	testNetwork.configuration = configuration
	testNetwork.initAppNetwork(ctx)
	return testNetwork
}

func (n *TestNetwork) AddVM(ctx context.Context, configBytes []byte) *VM {
	vm := NewTestVM(ctx, n.t, n.chainID, n.genesisBytes, configBytes)
	n.VMs = append(n.VMs, vm)
	n.nodeIDToVM[vm.nodeID] = vm
	n.uris = append(n.uris, vm.server.URL)
	n.initVMNetwork(ctx, vm)
	return vm
}

// initVMNetwork updates the AppSender of the provided VM to invoke the AppHandler interface
// of each VM in the test network.
func (n *TestNetwork) initVMNetwork(ctx context.Context, vm *VM) {
	myNodeID := vm.nodeID
	vm.appSender.SendAppRequestF = func(ctx context.Context, nodeIDs set.Set[ids.NodeID], u uint32, b []byte) error {
		go func() {
			for nodeID := range nodeIDs {
				if nodeID == myNodeID {
					vm.snowCtx.Lock.Lock()
					err := vm.SnowVM.AppRequest(ctx, myNodeID, u, time.Now().Add(time.Second), b)
					vm.snowCtx.Lock.Unlock()
					n.require.NoError(err)
				} else {
					peerVM := n.nodeIDToVM[nodeID]
					peerVM.snowCtx.Lock.Lock()
					err := peerVM.SnowVM.AppRequest(ctx, myNodeID, u, time.Now().Add(time.Second), b)
					peerVM.snowCtx.Lock.Unlock()
					n.require.NoError(err)
				}
			}
		}()
		return nil
	}
	vm.appSender.SendAppResponseF = func(ctx context.Context, nodeID ids.NodeID, requestID uint32, response []byte) error {
		go func() {
			if nodeID == myNodeID {
				vm.snowCtx.Lock.Lock()
				err := vm.SnowVM.AppResponse(ctx, myNodeID, requestID, response)
				vm.snowCtx.Lock.Unlock()
				n.require.NoError(err)
				return
			}

			peerVM := n.nodeIDToVM[nodeID]
			peerVM.snowCtx.Lock.Lock()
			err := peerVM.SnowVM.AppResponse(ctx, myNodeID, requestID, response)
			peerVM.snowCtx.Lock.Unlock()
			n.require.NoError(err)
		}()
		return nil
	}
	vm.appSender.SendAppErrorF = func(ctx context.Context, nodeID ids.NodeID, requestID uint32, code int32, message string) error {
		go func() {
			if nodeID == myNodeID {
				vm.snowCtx.Lock.Lock()
				err := vm.SnowVM.AppRequestFailed(ctx, myNodeID, requestID, &common.AppError{
					Code:    code,
					Message: message,
				})
				vm.snowCtx.Lock.Unlock()
				n.require.NoError(err)
				return
			}
			peerVM := n.nodeIDToVM[nodeID]
			peerVM.snowCtx.Lock.Lock()
			err := peerVM.SnowVM.AppRequestFailed(ctx, myNodeID, requestID, &common.AppError{
				Code:    code,
				Message: message,
			})
			peerVM.snowCtx.Lock.Unlock()
			n.require.NoError(err)
		}()
		return nil
	}
	vm.appSender.SendAppGossipF = func(ctx context.Context, sendConfig common.SendConfig, b []byte) error {
		nodeIDs := sendConfig.NodeIDs
		nodeIDs.Remove(myNodeID)
		// Select numSend nodes excluding myNodeID and gossip to them
		numSend := sendConfig.Validators + sendConfig.NonValidators + sendConfig.Peers
		sampledNodes := set.NewSet[ids.NodeID](numSend)
		for nodeID := range n.nodeIDToVM {
			if nodeID == myNodeID {
				continue
			}
			sampledNodes.Add(nodeID)
			if sampledNodes.Len() >= numSend {
				break
			}
		}
		nodeIDs.Union(sampledNodes)

		// Send to specified nodes
		for nodeID := range nodeIDs {
			peerVM := n.nodeIDToVM[nodeID]
			peerVM.snowCtx.Lock.Lock()
			err := peerVM.SnowVM.AppGossip(ctx, myNodeID, b)
			peerVM.snowCtx.Lock.Unlock()
			n.require.NoError(err)
		}
		return nil
	}

	for _, peer := range n.VMs {
		// Skip connecting to myself
		if peer.nodeID == vm.nodeID {
			continue
		}

		n.require.NoError(vm.SnowVM.Connected(ctx, peer.nodeID, version.CurrentApp))
		n.require.NoError(peer.SnowVM.Connected(ctx, vm.nodeID, version.CurrentApp))
	}
}

// initAppNetwork connects the AppSender / AppHandler interfaces of each VM in the
// test network
func (n *TestNetwork) initAppNetwork(ctx context.Context) {
	for _, vm := range n.VMs {
		n.initVMNetwork(ctx, vm)
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

// SubmitTxs submits the provided transactions to the first VM in the network and requires that no errors occur
func (n *TestNetwork) SubmitTxs(ctx context.Context, txs []*chain.Transaction) {
	// Submit tx to first node
	err := errors.Join(n.VMs[0].VM.Submit(ctx, txs)...)
	n.require.NoError(err)
}

// BuildBlockAndUpdateHead builds a block on the first VM, verifies the block and sets the initial VM's preference to the
// block.
// Every other VM in the network parses, verifies, and sets its own preference to this block.
// This function assumes that all VMs in the TestNetwork have verified at least up to the current preference
// of the initial VM, so that it correctly mimics the engine's behavior of guaranteeing that all VMs have
// verified the parent block before verifying its child.
func (n *TestNetwork) BuildBlockAndUpdateHead(ctx context.Context) []*snow.StatefulBlock[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock] {
	n.require.NoError(n.VMs[0].VM.Builder().Force(ctx))
	select {
	case <-n.VMs[0].toEngine:
	case <-time.After(time.Second):
		n.require.Fail("timeout waiting for PendingTxs message")
	}

	buildVM := n.VMs[0]
	blk := buildVM.BuildAndSetPreference(ctx)

	blks := make([]*snow.StatefulBlock[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock], len(n.VMs))
	blks[0] = blk
	blkBytes := blk.Bytes()
	for i, otherVM := range n.VMs[1:] {
		parsedBlk := otherVM.ParseAndSetPreference(ctx, blkBytes)
		parsedBlk, err := otherVM.SnowVM.GetCovariantVM().ParseBlock(ctx, blk.Bytes())
		n.require.NoError(err)
		n.require.Equal(blk.ID(), parsedBlk.ID())
		blks[i+1] = parsedBlk
	}

	return blks
}

func (n *TestNetwork) ConfirmTxsInBlock(ctx context.Context, blk *chain.ExecutionBlock, txs []*chain.Transaction) {
	for i, tx := range txs {
		n.require.True(blk.ContainsTx(tx.GetID()), "block does not contain tx %s at index %d", tx.GetID(), i)
	}
}

func (n *TestNetwork) ConfirmTxs(ctx context.Context, txs []*chain.Transaction) error {
	n.SubmitTxs(ctx, txs)
	blks := n.BuildBlockAndUpdateHead(ctx)

	outputBlock := blks[0].Output
	n.require.NotNil(outputBlock)

	n.ConfirmTxsInBlock(ctx, outputBlock.ExecutionBlock, txs)

	for i, blk := range blks {
		n.VMs[i].snowCtx.Lock.Lock()
		err := blk.Accept(ctx)
		n.VMs[i].snowCtx.Lock.Unlock()
		n.require.NoError(err, "failed to accept block at VM index %d", i)
	}
	return nil
}

func (n *TestNetwork) GenerateTx(ctx context.Context, actions []chain.Action, authFactory chain.AuthFactory) (*chain.Transaction, error) {
	cli := jsonrpc.NewJSONRPCClient(n.VMs[0].server.URL)
	_, tx, _, err := cli.GenerateTransaction(ctx, n.VMs[0].VM, actions, authFactory)
	return tx, err
}

func (n *TestNetwork) ConfirmBlocks(ctx context.Context, numBlocks int, generateTxs func(i int) []*chain.Transaction) {
	for i := 0; i < numBlocks; i++ {
		n.require.NoError(n.ConfirmTxs(ctx, generateTxs(i)))
	}
}

// SynchronizeNetwork picks the VM with the highest accepted block and synchronizes all other VMs to that block.
func (n *TestNetwork) SynchronizeNetwork(ctx context.Context) {
	var (
		greatestHeight      uint64
		greatestHeightIndex int
	)
	for i, vm := range n.VMs {
		height := vm.SnowVM.GetChainIndex().GetLastAccepted(ctx).Height()
		if height >= greatestHeight {
			greatestHeightIndex = i
			greatestHeight = height
		}
	}

	greatestVM := n.VMs[greatestHeightIndex]
	greatestVMChainIndex := greatestVM.SnowVM.GetChainIndex()

	for i, vm := range n.VMs {
		if i == greatestHeightIndex {
			continue
		}

		lastAcceptedHeight := vm.SnowVM.GetChainIndex().GetLastAccepted(ctx).Height()
		for lastAcceptedHeight < greatestHeight {
			blk, err := greatestVMChainIndex.GetBlockByHeight(ctx, lastAcceptedHeight+1)
			n.require.NoError(err)

			parsedBlk := vm.ParseAndSetPreference(ctx, blk.Bytes())
			n.require.NoError(parsedBlk.Accept(ctx))
			lastAcceptedHeight++
		}
	}
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
						Timestamp: utils.UnixRMilli(time.Now().UnixMilli(), 1_000),
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
	for _, vm := range network.VMs {
		vm.SnowVM.SetState(ctx, avasnow.NormalOp)
	}

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
	genesisBlock := network.VMs[0].SnowVM.GetChainIndex().GetLastAccepted(ctx)
	r.NoError(err)
	r.Equal(genesisBlock.ID(), blockID)
	r.Equal(uint64(0), blockHeight)
	r.Equal(genesisBlock.Timestamp(), timestamp)

	tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.authFactory)
	r.NoError(err)
	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx}))

	blockID, blockHeight, timestamp, err = client.Accepted(ctx)
	r.NoError(err)
	blk1 := network.VMs[0].SnowVM.GetChainIndex().GetLastAccepted(ctx)
	r.NoError(err)
	r.Equal(blk1.ID(), blockID)
	r.Equal(uint64(1), blockHeight)
	r.Equal(blk1.Timestamp(), timestamp)
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

	// Confirm and throw away last accepted block on startup
	genesisBlkID := <-externalSubscriberAcceptedBlocksCh
	lastAccepted := network.VMs[0].SnowVM.GetChainIndex().GetLastAccepted(ctx)
	r.Equal(lastAccepted.ID(), genesisBlkID)

	tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.authFactory)
	r.NoError(err)
	r.NoError(network.ConfirmTxs(ctx, []*chain.Transaction{tx}))

	acceptedBlkID := <-externalSubscriberAcceptedBlocksCh
	r.Equal(network.VMs[0].SnowVM.GetChainIndex().GetLastAccepted(ctx).ID(), acceptedBlkID)
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
	// XXX: re-write or remove ws server. RegisterTx is async, so we SHOULD
	// need to run a goroutine in this test to either confirm the block or
	// confirm receipt of the tx/block on the ws client. However, implementing it
	// this way does not cause the test to pass, whereas adding this sleep does.
	// Including the integration test until we remove or re-write the ws client/server.
	t.Skip()
	ctx := context.Background()
	r := require.New(t)
	chainID := ids.GenerateTestID()
	network := NewTestNetwork(ctx, t, chainID, 1, nil)

	client, err := ws.NewWebSocketClient(network.VMs[0].server.URL, time.Second, 10, consts.NetworkSizeLimit)
	defer func() {
		r.NoError(client.Close())
	}()
	r.NoError(err)

	r.NoError(client.RegisterBlocks())

	tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
		NumComputeUnits: 1,
	}}, network.authFactory)
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

	lastAccepted := network.VMs[0].SnowVM.GetChainIndex().GetLastAccepted(ctx)
	r.Equal(lastAccepted.ID(), wsBlk.ID())
	r.Equal(lastAccepted.ExecutionResults.Results, wsResults)
	r.Equal(lastAccepted.ExecutionResults.UnitPrices, wsUnitPrices)
}

func TestSkipStateSync(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	chainID := ids.GenerateTestID()
	network := NewTestNetwork(ctx, t, chainID, 1, nil)

	initialVM := network.VMs[0]
	initialVMGenesisBlock := initialVM.SnowVM.GetChainIndex().GetLastAccepted(ctx)

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
		}}, network.authFactory)
		r.NoError(err)
		return []*chain.Transaction{tx}
	})

	config := map[string]interface{}{
		vm.StateSyncNamespace: vm.StateSyncConfig{
			MinBlocks: uint64(numBlocks + 1),
		},
	}
	configBytes, err := json.Marshal(config)
	r.NoError(err)

	vm := network.AddVM(ctx, configBytes)
	lastAccepted := vm.SnowVM.GetChainIndex().GetLastAccepted(ctx)
	r.Equal(lastAccepted.ID(), initialVMGenesisBlock.ID())

	stateSummary, err := initialVM.SnowVM.GetLastStateSummary(ctx)
	r.NoError(err)
	parsedStateSummary, err := vm.SnowVM.ParseStateSummary(ctx, stateSummary.Bytes())
	r.NoError(err)

	// Accepting the state sync summary returns Skipped and does not change
	// the last accepted block.
	stateSyncMode, err := parsedStateSummary.Accept(ctx)
	r.NoError(err)
	r.Equal(block.StateSyncSkipped, stateSyncMode)
	r.Equal(lastAccepted.ID(), initialVMGenesisBlock.ID())

	network.SynchronizeNetwork(ctx)
	blk := vm.SnowVM.GetChainIndex().GetLastAccepted(ctx)
	r.Equal(blk.Height(), uint64(numBlocks))
}

func TestStateSync(t *testing.T) {
	ctx := context.Background()
	r := require.New(t)
	chainID := ids.GenerateTestID()
	network := NewTestNetwork(ctx, t, chainID, 1, nil, WithRulesOverride(func(r *genesis.Rules) {
		r.ValidityWindow = time.Second.Milliseconds()
	}))

	initialVM := network.VMs[0]
	initialVMGenesisBlock := initialVM.SnowVM.GetChainIndex().GetLastAccepted(ctx)

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
		}}, network.authFactory)
		r.NoError(err)
		return []*chain.Transaction{tx}
	})

	config := map[string]interface{}{
		vm.StateSyncNamespace: vm.StateSyncConfig{
			MinBlocks:                   uint64(numBlocks - 1),
			MerkleSimultaneousWorkLimit: 1, // TODO: this should not be required
		},
	}
	configBytes, err := json.Marshal(config)
	r.NoError(err)

	vm := network.AddVM(ctx, configBytes)
	lastAccepted := vm.SnowVM.GetChainIndex().GetLastAccepted(ctx)
	r.Equal(lastAccepted.ID(), initialVMGenesisBlock.ID())

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
	go func() {
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
				}}, network.authFactory)
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

	err = <-vm.VM.SyncClient.Done()
	close(stopCh)
	r.NoError(err)

	network.ConfirmBlocks(ctx, 1, func(i int) []*chain.Transaction {
		nonce++
		tx, err := network.GenerateTx(ctx, []chain.Action{&chaintest.TestAction{
			NumComputeUnits: 1,
			Nonce:           nonce,
		}}, network.authFactory)
		r.NoError(err)
		return []*chain.Transaction{tx}
	})
}

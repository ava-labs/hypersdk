// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vmtest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/enginetest"
	"github.com/ava-labs/avalanchego/snow/snowtest"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/version"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/api/jsonrpc"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/genesis"
	"github.com/ava-labs/hypersdk/snow"
	"github.com/ava-labs/hypersdk/tests/workload"
	"github.com/ava-labs/hypersdk/vm"

	avasnow "github.com/ava-labs/avalanchego/snow"
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
	vmFactory *vm.Factory,
	genesisBytes []byte,
	upgradeBytes []byte,
	configBytes []byte,
) *VM {
	r := require.New(t)
	vm, err := vmFactory.New()
	r.NoError(err)
	snowVM := snow.NewVM("v0.0.1", vm)

	toEngine := make(chan common.Message, 1)

	chainID := hashing.ComputeHash256Array(genesisBytes)
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
	r.NoError(snowVM.Initialize(ctx, snowCtx, nil, genesisBytes, upgradeBytes, configBytes, toEngine, nil, appSender))

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

	blk, err := vm.SnowVM.BuildBlock(ctx)
	vm.require.NoError(err)

	vm.require.NoError(blk.Verify(ctx))
	vm.require.NoError(vm.SnowVM.SetPreference(ctx, blk.ID()))
	return blk
}

func (vm *VM) ParseAndSetPreference(ctx context.Context, bytes []byte) *snow.StatefulBlock[*chain.ExecutionBlock, *chain.OutputBlock, *chain.OutputBlock] {
	vm.snowCtx.Lock.Lock()
	defer vm.snowCtx.Lock.Unlock()

	blk, err := vm.SnowVM.ParseBlock(ctx, bytes)
	vm.require.NoError(err)

	vm.require.NoError(blk.Verify(ctx))
	vm.require.NoError(vm.SnowVM.SetPreference(ctx, blk.ID()))
	return blk
}

func (vm *VM) ParseAndVerifyInvalidBlk(ctx context.Context, blkBytes []byte, expectedErr error) {
	vm.snowCtx.Lock.Lock()
	defer vm.snowCtx.Lock.Unlock()

	// Invalid block should fail during either parsing or verification.
	blk, err := vm.SnowVM.ParseBlock(ctx, blkBytes)
	if err != nil {
		vm.require.ErrorIs(err, expectedErr)
		return
	}

	vm.require.ErrorIs(blk.Verify(ctx), expectedErr)
}

type TestNetwork struct {
	t       *testing.T
	require *require.Assertions

	chainID       ids.ID
	genesisBytes  []byte
	upgradeBytes  []byte
	authFactories []chain.AuthFactory

	factory    *vm.Factory
	VMs        []*VM
	nodeIDToVM map[ids.NodeID]*VM
	uris       []string

	configuration workload.TestNetworkConfiguration
}

func NewTestNetwork(
	ctx context.Context,
	t *testing.T,
	factory *vm.Factory,
	genesisAndRuleFactory genesis.GenesisAndRuleFactory,
	numVMs int,
	authFactories []chain.AuthFactory,
	genesisBytes []byte,
	upgradeBytes []byte,
	configBytes []byte,
) *TestNetwork {
	r := require.New(t)
	chainID := hashing.ComputeHash256Array(genesisBytes)

	testNetwork := &TestNetwork{
		t:             t,
		require:       r,
		factory:       factory,
		chainID:       chainID,
		genesisBytes:  genesisBytes,
		upgradeBytes:  upgradeBytes,
		authFactories: authFactories,
	}

	vms := make([]*VM, numVMs)
	nodeIDToVM := make(map[ids.NodeID]*VM)
	uris := make([]string, len(vms))
	for i := range vms {
		vm := NewTestVM(ctx, t, factory, genesisBytes, upgradeBytes, configBytes)
		vms[i] = vm
		uris[i] = vm.server.URL
		nodeIDToVM[vm.nodeID] = vm
	}
	testNetwork.VMs = vms
	testNetwork.nodeIDToVM = nodeIDToVM
	testNetwork.uris = uris
	configuration := workload.NewDefaultTestNetworkConfiguration(
		"hypervmtests",
		genesisAndRuleFactory,
		vms[0].VM.GenesisBytes,
		vms[0].VM.GetParser(),
		authFactories,
	)
	testNetwork.configuration = configuration
	testNetwork.initAppNetwork(ctx)
	return testNetwork
}

func (n *TestNetwork) AddVM(ctx context.Context, configBytes []byte) *VM {
	vm := NewTestVM(ctx, n.t, n.factory, n.genesisBytes, n.upgradeBytes, configBytes)
	n.VMs = append(n.VMs, vm)
	n.nodeIDToVM[vm.nodeID] = vm
	n.uris = append(n.uris, vm.server.URL)
	n.initVMNetwork(ctx, vm)
	return vm
}

func (n *TestNetwork) SetState(ctx context.Context, state avasnow.State) {
	for _, vm := range n.VMs {
		n.require.NoError(vm.SnowVM.SetState(ctx, state))
	}
}

func (n *TestNetwork) Shutdown(ctx context.Context) {
	for _, vm := range n.VMs {
		vm.server.Close()
		n.require.NoError(vm.SnowVM.Shutdown(ctx))
	}
}

func (n *TestNetwork) ChainID() ids.ID { return n.chainID }

func (n *TestNetwork) AuthFactories() []chain.AuthFactory { return n.authFactories }

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
// Assumes that we can build a block without first submitting a valid tx
func (n *TestNetwork) ConfirmInvalidTx(ctx context.Context, invalidTx *chain.Transaction, targetErr error) {
	err := n.VMs[0].VM.SubmitTx(ctx, invalidTx)
	n.require.ErrorIs(err, targetErr)

	// Build on top of preferred block identical to how SubmitTx will simulate on the preferred block
	parentBlk, err := n.VMs[0].SnowVM.GetConsensusIndex().GetPreferredBlock(ctx)
	n.require.NoError(err)

	// Create alternative block with invalid tx.
	parentRoot, err := parentBlk.View.GetMerkleRoot(ctx)
	n.require.NoError(err)
	invalidBlk, err := chain.NewStatelessBlock(
		parentBlk.GetID(),
		max(time.Now().UnixMilli(), parentBlk.Tmstmp),
		parentBlk.Hght+1,
		[]*chain.Transaction{invalidTx},
		parentRoot,
		nil,
	)
	n.require.NoError(err)

	n.VMs[0].ParseAndVerifyInvalidBlk(ctx, invalidBlk.GetBytes(), targetErr)
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
		n.require.Equal(blk.ID(), parsedBlk.ID())
		blks[i+1] = parsedBlk
	}

	return blks
}

func (n *TestNetwork) ConfirmTxsInBlock(_ context.Context, blk *chain.ExecutionBlock, txs []*chain.Transaction) {
	for i, tx := range txs {
		n.require.True(blk.Contains(tx.GetID()), "block does not contain tx %s at index %d", tx.GetID(), i)
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
	unitPrices, err := cli.UnitPrices(ctx, true)
	if err != nil {
		return nil, err
	}
	return chain.GenerateTransaction(
		n.VMs[0].VM.GetRuleFactory(),
		unitPrices,
		time.Now().UnixMilli(),
		actions,
		authFactory,
	)
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
		lastAccepted, err := vm.SnowVM.GetConsensusIndex().GetLastAccepted(ctx)
		n.require.NoError(err)
		height := lastAccepted.GetHeight()
		if height >= greatestHeight {
			greatestHeightIndex = i
			greatestHeight = height
		}
	}

	greatestVM := n.VMs[greatestHeightIndex]
	greatestVMChainIndex := greatestVM.SnowVM.GetConsensusIndex()

	for i, vm := range n.VMs {
		if i == greatestHeightIndex {
			continue
		}

		lastAccepted, err := vm.SnowVM.GetConsensusIndex().GetLastAccepted(ctx)
		n.require.NoError(err)
		lastAcceptedHeight := lastAccepted.GetHeight()
		for lastAcceptedHeight < greatestHeight {
			blk, err := greatestVMChainIndex.GetBlockByHeight(ctx, lastAcceptedHeight+1)
			n.require.NoError(err)

			parsedBlk := vm.ParseAndSetPreference(ctx, blk.GetBytes())
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

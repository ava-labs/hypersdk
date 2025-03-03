// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/enginetest"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

// The nodes have partial state, we're testing client's functionality to query different nodes
// and construct valid state from partial states
func TestBlockFetcherClient_FetchBlocks_PartialAndComplete(t *testing.T) {
	req := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	validChain := generateTestChain(10)
	nodes := []nodeScenario{
		{
			blocks: map[uint64]ExecutionBlock[container]{
				0: validChain[0],
				1: validChain[1],
				2: validChain[2],
				3: validChain[3],
				4: validChain[4],
				5: validChain[5],
			},
		},
		{
			blocks: map[uint64]ExecutionBlock[container]{
				6: validChain[6],
				7: validChain[7],
				8: validChain[8],
				9: validChain[9],
			},
		},
	}

	network := setupTestNetwork(t, ctx, nodes)
	blkParser := setupParser(validChain)
	fetcher := NewBlockFetcherClient[ExecutionBlock[container]](network.client, blkParser, network.sampler)

	// start fetching from the tip i.e. last known accepted block, this block will not be included
	tip := validChain[len(validChain)-1]
	var minTS atomic.Int64
	minTS.Store(3)

	// Get block channel from fetcher
	resultChan := fetcher.FetchBlocks(ctx, tip, &minTS)

	// Collect all blocks from the channel
	receivedBlocks := make(map[uint64]ExecutionBlock[container])
	for result := range resultChan {
		block := result.Block
		receivedBlocks[block.GetHeight()] = block
	}

	req.Len(receivedBlocks, 7) // block height from 8 to 2 should be received; 2 being boundary block due to strict verification
	for _, block := range validChain[2:9] {
		received, ok := receivedBlocks[block.GetHeight()]
		req.True(ok)
		req.Equal(block.GetID(), received.GetID())
	}
}

func TestBlockFetcherClient_MaliciousNode(t *testing.T) {
	req := require.New(t)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
	defer cancel()

	chain := generateTestChain(20)

	nodes := []nodeScenario{
		{
			// First node has almost full state of valid transactions
			blocks: map[uint64]ExecutionBlock[container]{
				0: chain[2],
				1: chain[5],
				2: chain[9],
				3: chain[11],
				4: chain[4],
				5: chain[5],
				6: chain[6],
				7: chain[7],
				8: chain[8],
				9: chain[9],
			},
		},
	}

	network := setupTestNetwork(t, ctx, nodes)
	blockValidator := setupParser(chain)
	fetcher := NewBlockFetcherClient[ExecutionBlock[container]](network.client, blockValidator, network.sampler)
	tip := chain[9]
	var minTS atomic.Int64
	minTS.Store(3)

	resultChan := fetcher.FetchBlocks(ctx, tip, &minTS)
	receivedBlocks := make(map[uint64]ExecutionBlock[container])

	// upper bound timeout for receiving block
	timeout := time.After(5 * time.Second)
loop:
	for {
		select {
		case result := <-resultChan:
			block := result.Block
			receivedBlocks[block.GetHeight()] = block
			// reset the timeout for each successful block received
			timeout = time.After(5 * time.Second)
		case <-timeout:
			break loop
		}
	}

	// We should have 5 blocks in our state instead of 6 since the last one is invalid. Partial commit of valid blocks
	req.Len(receivedBlocks, 5)

	// Verify blocks from 8 to 4 have been received
	for i := 4; i <= 8; i++ {
		expectedBlock := chain[i]
		h := expectedBlock.GetHeight()
		receivedBlock, ok := receivedBlocks[h]
		req.True(ok)
		req.Equal(expectedBlock, receivedBlock)
	}

	// Invalid blocks should not be received
	for _, invalidBlock := range chain[:4] {
		_, ok := receivedBlocks[invalidBlock.GetHeight()]
		req.False(ok)
	}
}

/*
Test demonstrates dynamic minTimestamp boundary updates during block fetching.

Initial state:

	Blocks (height -> minTimestamp):  0->0, 1->1, 2->2, 3->3, 4->4, 5->5, 6->6, 7->7, 8->8, 9->9
	Initial minTS = 3, blkHeight=9: Should fetch blocks with timestamps > 3 (blocks 4-8)

During execution:
1. Node responds with a delay for each request
2. After fetching some blocks minTS is updated to 5
3. This updates the boundary - now only fetches blocks with timestamps > 5

Expected outcome:
  - Only blocks 4 to 8 should be received (5 blocks total, 4th block being boundary block due to stricter adherence)
  - Blocks 0-3 should not be received as they're below the updated minTS
*/
func TestBlockFetcherClient_FetchBlocksChangeOfTimestamp(t *testing.T) {
	req := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	delay := 50 * time.Millisecond
	validChain := generateTestChain(10)
	nodes := []nodeScenario{
		{
			blocks: map[uint64]ExecutionBlock[container]{
				0: validChain[0],
				1: validChain[1],
				2: validChain[2],
				3: validChain[3],
				4: validChain[4],
				5: validChain[5],
				6: validChain[6],
				7: validChain[7],
				8: validChain[8],
				9: validChain[9],
			},
			responseDelay: delay,
		},
	}

	network := setupTestNetwork(t, ctx, nodes)
	blkParser := setupParser(validChain)
	fetcher := NewBlockFetcherClient[ExecutionBlock[container]](network.client, blkParser, network.sampler)

	tip := validChain[len(validChain)-1]

	var minTimestamp atomic.Int64
	minTimestamp.Store(3)
	go func() {
		time.Sleep(delay * 2)
		minTimestamp.Store(5)
	}()

	resultChan := fetcher.FetchBlocks(ctx, tip, &minTimestamp)

	receivedBlocks := make(map[uint64]ExecutionBlock[container])
	for result := range resultChan {
		block := result.Block
		receivedBlocks[block.GetHeight()] = block
	}

	req.Len(receivedBlocks, 5)
	for _, block := range validChain[4:9] {
		_, ok := receivedBlocks[block.GetHeight()]
		req.True(ok)
	}

	for _, block := range validChain[:4] {
		_, ok := receivedBlocks[block.GetHeight()]
		req.False(ok)
	}
}

func generateTestChain(n int) []ExecutionBlock[container] {
	chain := generateBlockChain(n, 5)
	validChain := make([]ExecutionBlock[container], 0, len(chain))
	for _, block := range chain {
		validChain = append(validChain, block)
	}

	sort.Slice(validChain, func(i, j int) bool {
		return validChain[i].GetHeight() < validChain[j].GetHeight()
	})

	return validChain
}

// === Test Helpers ===
type nodeScenario struct {
	blocks        map[uint64]ExecutionBlock[container] // in-memory blocks a node might have
	responseDelay time.Duration
}

type testNetwork struct {
	client  *p2p.Client
	sampler *testNodeSampler
	nodes   []ids.NodeID
}

func setupTestNetwork(t *testing.T, ctx context.Context, nodeScenarios []nodeScenario) *testNetwork {
	clientNodeID := ids.GenerateTestNodeID()
	handlers := make(map[ids.NodeID]p2p.Handler)
	nodes := make([]ids.NodeID, len(nodeScenarios))

	for _, scenario := range nodeScenarios {
		nodeID := ids.GenerateTestNodeID()
		nodes = append(nodes, nodeID)

		blkRetriever := newTestBlockRetriever().withBlocks(scenario.blocks).withNodeID(nodeID)
		if scenario.responseDelay > 0 {
			blkRetriever.withDelay(scenario.responseDelay)
		}

		handlers[nodeID] = NewBlockFetcherHandler(blkRetriever)
	}

	return &testNetwork{
		client:  newClientWithPeers(t, ctx, clientNodeID, p2p.NoOpHandler{}, handlers),
		sampler: &testNodeSampler{nodes: nodes},
		nodes:   nodes,
	}
}

// === Setups ===
var (
	_ BlockParser[Block] = (*parser[Block])(nil)
	_ p2p.NodeSampler    = (*testNodeSampler)(nil)
)

// === BlockParser ===
// parser implements BlockParser
type parser[T Block] struct {
	parseErr error
	state    map[ids.ID]T
}

func (m *parser[T]) ParseBlock(_ context.Context, data []byte) (T, error) {
	if m.parseErr != nil {
		return utils.Zero[T](), m.parseErr
	}

	var blockID ids.ID
	copy(blockID[:], data)

	block, ok := m.state[blockID]
	if !ok {
		return utils.Zero[T](), fmt.Errorf("block %s not found", blockID)
	}
	return block, nil
}

// setupParser is prefilling the parser state, mocking parsing
func setupParser(chain []ExecutionBlock[container]) *parser[ExecutionBlock[container]] {
	p := &parser[ExecutionBlock[container]]{
		state: make(map[ids.ID]ExecutionBlock[container]),
	}

	for _, block := range chain {
		p.state[block.GetID()] = block
	}

	return p
}

// === NODE SAMPLER ===
type testNodeSampler struct {
	nodes []ids.NodeID
}

func (t *testNodeSampler) Sample(_ context.Context, num int) []ids.NodeID {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec
	r.Shuffle(len(t.nodes), func(i, j int) {
		t.nodes[i], t.nodes[j] = t.nodes[j], t.nodes[i]
	})
	if len(t.nodes) < num {
		return t.nodes
	}
	return t.nodes[:num]
}

// newClientWithPeers generates a client to communicate to a set of peers
func newClientWithPeers(
	t *testing.T,
	ctx context.Context,
	clientNodeID ids.NodeID,
	clientHandler p2p.Handler,
	peers map[ids.NodeID]p2p.Handler,
) *p2p.Client {
	peers[clientNodeID] = clientHandler

	peerSenders := make(map[ids.NodeID]*enginetest.Sender)
	peerNetworks := make(map[ids.NodeID]*p2p.Network)
	for nodeID := range peers {
		peerSenders[nodeID] = &enginetest.Sender{}
		peerNetwork, err := p2p.NewNetwork(logging.NoLog{}, peerSenders[nodeID], prometheus.NewRegistry(), "")
		require.NoError(t, err)
		peerNetworks[nodeID] = peerNetwork
	}

	peerSenders[clientNodeID].SendAppGossipF = func(ctx context.Context, _ common.SendConfig, gossipBytes []byte) error {
		// Send the request asynchronously to avoid deadlock when the server
		// sends the response back to the client
		for nodeID := range peers {
			go func() {
				require.NoError(t, peerNetworks[nodeID].AppGossip(ctx, nodeID, gossipBytes))
			}()
		}

		return nil
	}

	peerSenders[clientNodeID].SendAppRequestF = func(ctx context.Context, nodeIDs set.Set[ids.NodeID], requestID uint32, requestBytes []byte) error {
		for nodeID := range nodeIDs {
			network, ok := peerNetworks[nodeID]
			if !ok {
				return fmt.Errorf("%s is not connected", nodeID)
			}

			// Send the request asynchronously to avoid deadlock when the server
			// sends the response back to the client
			go func() {
				require.NoError(t, network.AppRequest(ctx, clientNodeID, requestID, time.Time{}, requestBytes))
			}()
		}

		return nil
	}

	for nodeID := range peers {
		peerSenders[nodeID].SendAppResponseF = func(ctx context.Context, _ ids.NodeID, requestID uint32, responseBytes []byte) error {
			// Send the request asynchronously to avoid deadlock when the server
			// sends the response back to the client
			go func() {
				require.NoError(t, peerNetworks[clientNodeID].AppResponse(ctx, nodeID, requestID, responseBytes))
			}()

			return nil
		}
	}

	for nodeID := range peers {
		peerSenders[nodeID].SendAppErrorF = func(ctx context.Context, _ ids.NodeID, requestID uint32, errorCode int32, errorMessage string) error {
			// Send the request asynchronously to avoid deadlock when the server
			// sends the response back to the client
			nodeIDCopy := nodeID
			go func() {
				require.NoError(t, peerNetworks[clientNodeID].AppRequestFailed(ctx, nodeIDCopy, requestID, &common.AppError{
					Code:    errorCode,
					Message: errorMessage,
				}))
			}()

			return nil
		}
	}

	for nodeID := range peers {
		require.NoError(t, peerNetworks[nodeID].Connected(ctx, clientNodeID, nil))
		require.NoError(t, peerNetworks[nodeID].Connected(ctx, nodeID, nil))
		require.NoError(t, peerNetworks[nodeID].AddHandler(0, peers[nodeID]))
	}

	return peerNetworks[clientNodeID].NewClient(0)
}

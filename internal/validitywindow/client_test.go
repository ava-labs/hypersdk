// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/network/p2p/p2ptest"
	"github.com/stretchr/testify/require"
)

// The nodes have partial state, we're testing client's functionality to query different nodes
// and construct valid state from partial states
func TestBlockFetcherClient_FetchBlocks_PartialAndComplete(t *testing.T) {
	req := require.New(t)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	validChain := generateChain(10)

	nodes := []nodeScenario{
		{
			blocks: map[uint64]*testBlock{
				0: validChain[0],
				1: validChain[1],
				2: validChain[2],
				3: validChain[3],
				4: validChain[4],
				5: validChain[5],
			},
		},
		{
			blocks: map[uint64]*testBlock{
				6: validChain[6],
				7: validChain[7],
				8: validChain[8],
				9: validChain[9],
			},
		},
	}

	network := setupTestNetwork(t, ctx, nodes)
	blockValidator := setupBlockProcessor(validChain)
	fetcher := NewBlockFetcherClient[*testBlock](network.client, blockValidator, network.sampler)

	tip := validChain[len(validChain)-1]
	var minTS atomic.Int64
	minTS.Store(3)
	err := fetcher.FetchBlocks(ctx, tip.GetID(), tip.GetHeight(), tip.GetTimestamp(), &minTS)
	req.NoError(err)
	req.Len(blockValidator.receivedBlocks, 7) // block height from 9 to 3 should be written

	for _, expectedWrittenBlock := range validChain[3:] {
		blockID := expectedWrittenBlock.GetID()
		writtenBlock, ok := blockValidator.knownBlocks[blockID]
		req.True(ok)
		req.Equal(expectedWrittenBlock, writtenBlock)
	}
}

func TestBlockFetcherClient_MaliciousNode(t *testing.T) {
	req := require.New(t)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
	defer cancel()

	validChain := generateChain(10)
	invalidChain := generateChain(10)

	nodes := []nodeScenario{
		{
			// First node has almost full state of valid transactions
			blocks: map[uint64]*testBlock{
				0: invalidChain[0],
				1: invalidChain[1],
				2: invalidChain[2],
				3: invalidChain[3],
				4: validChain[4],
				5: validChain[5],
				6: validChain[6],
				7: validChain[7],
				8: validChain[8],
				9: validChain[9],
			},
		},
	}

	network := setupTestNetwork(t, ctx, nodes)

	// We're backfilling mockBlockValidator's knownBlocks in order to pass ParseBlock
	all := make([]*testBlock, len(validChain)+len(invalidChain))
	copy(all, append(validChain, invalidChain...))
	blockValidator := setupBlockProcessor(all)

	fetcher := NewBlockFetcherClient[*testBlock](network.client, blockValidator, network.sampler)
	tip := validChain[len(validChain)-1]
	var minTS atomic.Int64
	minTS.Store(3)

	err := fetcher.FetchBlocks(ctx, tip.GetID(), tip.GetHeight(), tip.GetTimestamp(), &minTS)

	// Expecting a context.DeadlineExceeded error because the nodeScenario consists of only one node (we will be sampling only one node each iteration).
	// The node has a partially correct state but lacks the full validity window since the required third block is invalid.
	// As a result, the setup ensures that a valid block can never be found.
	req.ErrorIs(err, context.DeadlineExceeded)

	// We should have 6 blocks in our state instead of 7 since the last one is invalid. Partial commit of valid blocks
	req.Len(blockValidator.receivedBlocks, 6)

	// Verify blocks from 9 to 4 have been written
	for _, expectedWrittenBlock := range validChain[4:] {
		blockID := expectedWrittenBlock.GetID()
		writtenBlock, ok := blockValidator.knownBlocks[blockID]
		req.True(ok)
		req.Equal(expectedWrittenBlock, writtenBlock)
	}

	// Invalid blocks should not be written
	for _, invalidBlocks := range validChain[:4] {
		_, ok := blockValidator.receivedBlocks[invalidBlocks.GetID()]
		req.False(ok)
	}
}

/*
Test demonstrates dynamic minTimestamp boundary updates during block fetching.

Initial state:

	Blocks (height -> minTimestamp):  0->0, 1->1, 2->2, 3->3, 4->4, 5->5, 6->6, 7->7, 8->8, 9->9 -> 10->10 -> 11->11
	Initial minTS = 3: Should fetch blocks with timestamps > 3 (blocks 4-9)

During execution:
 1. Node responds with a delay for each request
 2. After fetching some blocks minTS is updated to 5
 3. This updates the boundary - now only fetches blocks with timestamps > 5

Expected outcome:
  - Only blocks 5-9 should be received (5 blocks total)
  - Blocks 0-4 should not be received as they're below the updated minTS
*/
func TestBlockFetcherClient_FetchBlocksChangeOfTimestamp(t *testing.T) {
	req := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	delay := 50 * time.Millisecond
	validChain := generateChain(10)
	nodes := []nodeScenario{
		{
			blocks: map[uint64]*testBlock{
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
	blockValidator := setupBlockProcessor(validChain)
	fetcher := NewBlockFetcherClient[*testBlock](network.client, blockValidator, network.sampler)

	tip := validChain[len(validChain)-1]

	var minTimestamp atomic.Int64
	minTimestamp.Store(3)
	go func() {
		time.Sleep(delay * 2)
		minTimestamp.Store(5)
	}()

	err := fetcher.FetchBlocks(ctx, tip.GetID(), tip.GetHeight(), tip.GetTimestamp(), &minTimestamp)
	req.NoError(err)
	req.Len(blockValidator.receivedBlocks, 5)

	for _, block := range validChain[5:] {
		_, ok := blockValidator.receivedBlocks[block.GetID()]
		req.True(ok)
	}

	for _, block := range validChain[:5] {
		_, ok := blockValidator.receivedBlocks[block.GetID()]
		req.False(ok)
	}
}

type testBlock struct {
	height    uint64
	id        ids.ID
	parentID  ids.ID
	timestamp int64
	bytes     []byte
}

func newTestBlock(id ids.ID, parentID ids.ID, height uint64, timestamp int64) *testBlock {
	return &testBlock{
		id:        id,
		parentID:  parentID,
		height:    height,
		timestamp: timestamp,
		bytes:     id[:],
	}
}

func (m *testBlock) GetID() ids.ID       { return m.id }
func (m *testBlock) GetParent() ids.ID   { return m.parentID }
func (m *testBlock) GetHeight() uint64   { return m.height }
func (m *testBlock) GetTimestamp() int64 { return m.timestamp }
func (m *testBlock) GetBytes() []byte    { return m.bytes }

func generateChain(numBlocks int) []*testBlock {
	if numBlocks <= 0 {
		return nil
	}

	chain := make([]*testBlock, numBlocks)

	genesisID := ids.GenerateTestID()
	chain[0] = newTestBlock(genesisID, ids.Empty, 0, 0)

	for i := 1; i < numBlocks; i++ {
		blockID := ids.GenerateTestID()
		parentID := chain[i-1].GetID()
		timestamp := chain[i-1].GetTimestamp()
		chain[i] = newTestBlock(blockID, parentID, uint64(i), timestamp+1)
	}

	return chain
}

// === Test Helpers ===
type nodeScenario struct {
	blocks        map[uint64]*testBlock // in-memory blocks a node might have
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
		client:  p2ptest.NewClientWithPeers(t, ctx, clientNodeID, p2p.NoOpHandler{}, handlers),
		sampler: &testNodeSampler{nodes: nodes},
		nodes:   nodes,
	}
}

// === Setups ===
var (
	_ BlockProcessor[*testBlock] = (*mockBlockProcessor)(nil)
	_ p2p.NodeSampler            = (*testNodeSampler)(nil)
)

// === BlockProcessor ===
// mockBlockProcessor implements BlockProcessor
type mockBlockProcessor struct {
	parseErr error
	writeErr error

	knownBlocks    map[ids.ID]*testBlock
	receivedBlocks map[ids.ID]*testBlock
}

func (m *mockBlockProcessor) ParseBlock(_ context.Context, data []byte) (*testBlock, error) {
	if m.parseErr != nil {
		return nil, m.parseErr
	}

	var blockID ids.ID
	copy(blockID[:], data)

	block, ok := m.knownBlocks[blockID]
	if !ok {
		return nil, fmt.Errorf("block %s not found", blockID)
	}
	return block, nil
}

func (m *mockBlockProcessor) WriteBlock(_ context.Context, block Block) error {
	if m.writeErr != nil {
		return m.writeErr
	}

	testBlock, ok := block.(*testBlock)
	if !ok {
		return fmt.Errorf("expected *TestBlock, got %T", block)
	}

	m.receivedBlocks[testBlock.GetID()] = testBlock
	return nil
}

func setupBlockProcessor(chain []*testBlock) *mockBlockProcessor {
	validator := &mockBlockProcessor{
		knownBlocks:    make(map[ids.ID]*testBlock),
		receivedBlocks: make(map[ids.ID]*testBlock),
	}

	for _, block := range chain {
		validator.knownBlocks[block.GetID()] = block
	}

	return validator
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

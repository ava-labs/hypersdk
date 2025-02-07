// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blockwindowsyncer

import (
	"context"
	"fmt"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/network/p2p/p2ptest"
	"github.com/ava-labs/hypersdk/blockwindowsyncer/blockwindowsyncertest"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// The nodes have partial state, we're testing client's functionality to query different nodes
// and construct valid state from partial states
func TestBlockFetcherClient_FetchBlocks_PartialAndComplete(t *testing.T) {
	req := require.New(t)
	ctx := context.Background()

	validChain := blockwindowsyncertest.GenerateChain(10)

	nodes := []nodeScenario{
		{
			blocks: map[uint64]*blockwindowsyncertest.TestBlock{
				0: validChain[0],
				1: validChain[1],
				2: validChain[2],
				3: validChain[3],
				4: validChain[4],
				5: validChain[5],
			},
		},
		{
			blocks: map[uint64]*blockwindowsyncertest.TestBlock{
				6: validChain[6],
				7: validChain[7],
				8: validChain[8],
				9: validChain[9],
			},
		},
	}

	network := setupTestNetwork(t, ctx, nodes)
	blockValidator := setupBlockValidator(validChain)
	fetcher := NewBlockFetcherClient[*blockwindowsyncertest.TestBlock](network.client, blockValidator, network.sampler)

	tip := validChain[len(validChain)-1]
	err := fetcher.FetchBlocks(ctx, tip, 3)
	req.NoError(err)
	req.Len(blockValidator.receivedBlocks, 7) // block height from 9 to 3 should be written

	for _, expectedWrittenBlock := range validChain[3:] {
		blockID := expectedWrittenBlock.GetID()
		writtenBlock, ok := blockValidator.knownBlocks[blockID]
		req.True(ok)
		req.Equal(writtenBlock, expectedWrittenBlock)
	}
}

func TestBlockFetcherClient_MaliciousNode(t *testing.T) {
	req := require.New(t)
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	validChain := blockwindowsyncertest.GenerateChain(10)
	invalidChain := blockwindowsyncertest.GenerateChain(10)

	// The node has almost full state of valid transactions except the last one (last one in order to fill validity window)
	nodes := []nodeScenario{
		{
			blocks: map[uint64]*blockwindowsyncertest.TestBlock{
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
	all := append(validChain, invalidChain...)
	blockValidator := setupBlockValidator(all)

	fetcher := NewBlockFetcherClient[*blockwindowsyncertest.TestBlock](network.client, blockValidator, network.sampler)
	tip := validChain[len(validChain)-1]
	err := fetcher.FetchBlocks(ctx, tip, 3)
	req.ErrorIs(err, errMaliciousNode)

	// We should have 6 blocks in our state instead of 7 since the last one is invalid
	req.Len(blockValidator.receivedBlocks, 6)

	// Verify blocks from 9 to 4 have been written
	for _, expectedWrittenBlock := range validChain[4:] {
		blockID := expectedWrittenBlock.GetID()
		writtenBlock, ok := blockValidator.knownBlocks[blockID]
		req.True(ok)
		req.Equal(writtenBlock, expectedWrittenBlock)
	}

	// Invalid blocks should not be written
	for _, invalidBlocks := range validChain[:4] {
		_, ok := blockValidator.receivedBlocks[invalidBlocks.GetID()]
		req.False(ok)
	}
}

type nodeScenario struct {
	blocks        map[uint64]*blockwindowsyncertest.TestBlock // in-memory blocks a node might have
	responseDelay time.Duration
}

type testNetwork struct {
	client  *p2p.Client
	sampler *blockwindowsyncertest.TestNodeSampler
	nodes   []ids.NodeID
}

func setupTestNetwork(t *testing.T, ctx context.Context, nodeScenarios []nodeScenario) *testNetwork {
	clientNodeID := ids.GenerateTestNodeID()
	handlers := make(map[ids.NodeID]p2p.Handler)
	nodes := make([]ids.NodeID, len(nodeScenarios))

	for i, scenario := range nodeScenarios {
		nodeID := ids.GenerateTestNodeID()
		nodes = append(nodes, nodeID)

		fmt.Printf("%d: %s\n", i+1, nodeID)

		blkRetriever := blockwindowsyncertest.NewTestBlockRetriever().WithBlocks(scenario.blocks).WithNodeID(nodeID)
		if scenario.responseDelay > 0 {
			blkRetriever.WithDelay(scenario.responseDelay)
		}

		handlers[nodeID] = NewBlockFetcherHandler(blkRetriever)
	}

	return &testNetwork{
		client:  p2ptest.NewClientWithPeers(t, ctx, clientNodeID, p2p.NoOpHandler{}, handlers),
		sampler: &blockwindowsyncertest.TestNodeSampler{Nodes: nodes},
		nodes:   nodes,
	}
}

// mockBlockValidator implements BlockParser
type mockBlockValidator struct {
	parseErr error
	writeErr error

	knownBlocks    map[ids.ID]*blockwindowsyncertest.TestBlock
	receivedBlocks map[ids.ID]*blockwindowsyncertest.TestBlock
}

func (m *mockBlockValidator) ParseBlock(_ context.Context, data []byte) (*blockwindowsyncertest.TestBlock, error) {
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

func (m *mockBlockValidator) WriteBlock(_ context.Context, blk *blockwindowsyncertest.TestBlock) error {
	if m.writeErr != nil {
		return m.writeErr
	}

	m.receivedBlocks[blk.GetID()] = blk
	//fmt.Printf("WriteBlock: height=%d id=%s\n", blk.GetHeight(), blk.GetID())
	return nil
}

func setupBlockValidator(chain []*blockwindowsyncertest.TestBlock) *mockBlockValidator {
	validator := &mockBlockValidator{
		knownBlocks:    make(map[ids.ID]*blockwindowsyncertest.TestBlock),
		receivedBlocks: make(map[ids.ID]*blockwindowsyncertest.TestBlock),
	}

	for _, block := range chain {
		validator.knownBlocks[block.GetID()] = block
	}

	return validator
}

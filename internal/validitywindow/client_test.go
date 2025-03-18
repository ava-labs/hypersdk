// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package validitywindow

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/stretchr/testify/require"
)

var (
	_          BlockParser[Block]  = (*parser[Block])(nil)
	_          p2p.NodeSampler     = (*controlledNodeSampler)(nil)
	_          NetworkBlockFetcher = (*testP2PBlockFetcher)(nil)
	timeout                        = 5 * time.Second
	errTimeout                     = errors.New("handler timeout")
)

// The nodes have partial state, we're testing client's functionality to query different nodes
// and construct valid state from partial states
func TestBlockFetcherClient_FetchBlocks_PartialAndComplete(t *testing.T) {
	r := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	validChain := generateTestChain(10)
	nodeScenarios := []nodeSetup{
		{
			sampleOrder: 1,
			blocks: map[uint64]ExecutionBlock[container]{
				6: validChain[6],
				7: validChain[7],
				8: validChain[8],
				9: validChain[9],
			},
		},
		{
			sampleOrder: 2,
			blocks: map[uint64]ExecutionBlock[container]{
				0: validChain[0],
				1: validChain[1],
				2: validChain[2],
				3: validChain[3],
				4: validChain[4],
				5: validChain[5],
			},
		},
	}

	test := newTestEnvironment(nodeScenarios)
	fetcher := NewBlockFetcherClient[ExecutionBlock[container]](
		test.p2pBlockFetcher,
		newParser(validChain),
		test.sampler,
	)

	// Start fetching from the tip
	tip := validChain[len(validChain)-1]

	var (
		minTS             atomic.Int64
		numExpectedBlocks = 7
	)
	minTS.Store(3)
	resultChan := fetcher.FetchBlocks(ctx, tip, &minTS)

	receivedBlocks, err := test.collectBlocksWithTimeout(ctx, resultChan, numExpectedBlocks)
	r.NoError(err)

	// Retrieve blocks from height 8 down to 2 (inclusive)
	//nolint:dupword
	// Block timeline:
	//
	//    blk 2    blk 3    blk 4    blk 5    blk 6    blk 7    blk 8
	//     |        |        |        |        |        |        |
	//     v        v        v        v        v        v        v
	// ----+--------+--------+--------+--------+--------+--------+--> time
	//     |        |
	//     |        |
	//     |        +---- Min timestamp requirement
	//     |              starts at this block
	//     |
	//     +---- Boundary block
	//           (first block with timestamp < minimum)
	//
	// We include block 2 due to strict verification requirements.
	// Even though our minimum timestamp starts at block 3,
	// we need the boundary block (block 2, the first block whose
	// timestamp is strictly less than our minimum) to properly
	// verify the entire validity window.
	r.Len(receivedBlocks, numExpectedBlocks, "Should receive 7 blocks")
	for _, block := range validChain[2:9] {
		received, ok := receivedBlocks[block.GetHeight()]
		r.True(ok, "Block at height %d should be received", block.GetHeight())
		r.Equal(block.GetID(), received.GetID())
	}
}

func TestBlockFetcherClient_MaliciousNode(t *testing.T) {
	r := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	chain := generateTestChain(20)
	numReceivedBlocks := 5

	nodeSetups := []nodeSetup{
		{
			sampleOrder: 1,
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
			blkControl: make(chan struct{}, numReceivedBlocks), // blocks from 8 to 4
		},
		{
			sampleOrder: 2,
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
			blkControl: make(chan struct{}),
		},
	}

	test := newTestEnvironment(nodeSetups)
	fetcher := NewBlockFetcherClient[ExecutionBlock[container]](test.p2pBlockFetcher, newParser(chain), test.sampler)

	var minTS atomic.Int64
	minTS.Store(3)

	tip := chain[9]
	resultChan := fetcher.FetchBlocks(ctx, tip, &minTS)

	receivedBlocks, err := test.collectBlocksWithTimeout(ctx, resultChan, numReceivedBlocks)
	// Due to the malicious node, we receive blocks 8-4 instead of 8-3.
	// The timeout error forces a partial but valid response.
	r.ErrorIs(err, errTimeout)

	r.Len(receivedBlocks, numReceivedBlocks)

	// Verify blocks from 8 to 4 have been received
	for i := 4; i <= 8; i++ {
		expectedBlock := chain[i]
		h := expectedBlock.GetHeight()
		receivedBlock, ok := receivedBlocks[h]
		r.True(ok)
		r.EqualValues(expectedBlock, receivedBlock)
	}
}

/*
Test demonstrates dynamic minTimestamp boundary updates during block fetching.

Initial state:

	Blocks (height -> minTimestamp):  0->0, 1->1, 2->2, 3->3, 4->4, 5->5, 6->6, 7->7, 8->8, 9->9
	Initial minTS = 3, blkHeight=9: Should fetch blocks with timestamps >= 2 and <9  (blocks 2-8)

During execution:
- After fetching some blocks, minTS is updated to 5
- This updates the boundary - now only fetches blocks with timestamps >= 4 and < 9

Expected outcome:
  - Only blocks 4 to 8 should be received (5 blocks total, 4th block being boundary block due to stricter adherence)
  - Blocks 0-3 should not be received as they're below the updated minTS
*/
func TestBlockFetcherClient_FetchBlocksChangeOfTimestamp(t *testing.T) {
	r := require.New(t)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	validChain := generateTestChain(10)
	nodeSetups := []nodeSetup{
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
		},
	}

	test := newTestEnvironment(nodeSetups)
	fetcher := NewBlockFetcherClient[ExecutionBlock[container]](test.p2pBlockFetcher, newParser(validChain), test.sampler)

	tip := validChain[len(validChain)-1]

	var (
		minTimestamp      atomic.Int64
		numExpectedBlocks = 5
	)
	minTimestamp.Store(3)

	// Signal timestamp update
	signalUpdate := make(chan struct{})
	go func() {
		select {
		case <-signalUpdate:
			minTimestamp.Store(5)
		case <-ctx.Done():
			return
		}
	}()

	resultChan := fetcher.FetchBlocks(ctx, tip, &minTimestamp)
	receivedBlocks := make(map[uint64]ExecutionBlock[container], numExpectedBlocks)

	success := make(chan struct{})
	go func() {
		recvCount := 0
		defer close(success)

		for blk := range resultChan {
			height := blk.GetHeight()
			receivedBlocks[height] = blk

			recvCount += 1
			// After receiving some blocks signal to update the timestamp
			if recvCount == 2 {
				close(signalUpdate)
			}
		}
	}()

	select {
	case <-success:
		r.Len(receivedBlocks, numExpectedBlocks)
		for _, block := range validChain[4:9] {
			_, ok := receivedBlocks[block.GetHeight()]
			r.True(ok)
		}
	case <-ctx.Done():
		r.Fail("context timeout")
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

type nodeSetup struct {
	nodeID ids.NodeID
	// blkControl is controlling how many block retrieval operations can occur.
	// With a buffer of N, exactly N blocks will be retrieved before blocking indefinitely,
	// allowing tests to create predictable timeout scenarios with partial results.
	blkControl  chan struct{}
	blocks      map[uint64]ExecutionBlock[container] // in-memory blocks a node might have
	sampleOrder uint8                                // Order in which this node should be sampled
}

type testP2PBlockFetcher struct {
	handlers map[ids.NodeID]*BlockFetcherHandler[ExecutionBlock[container]]
	err      chan error
}

func (t *testP2PBlockFetcher) FetchBlocksFromPeer(ctx context.Context, nodeID ids.NodeID, request *BlockFetchRequest) (*BlockFetchResponse, error) {
	handler, ok := t.handlers[nodeID]
	if !ok {
		return nil, fmt.Errorf("handler %s not found", nodeID)
	}

	blocks, err := handler.fetchBlocks(ctx, request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			t.err <- errTimeout
			return nil, errTimeout
		}
		return nil, err
	}

	return &BlockFetchResponse{Blocks: blocks}, nil
}

type testEnvironment struct {
	p2pBlockFetcher *testP2PBlockFetcher
	sampler         *controlledNodeSampler
	nodes           []ids.NodeID
}

func newTestEnvironment(nodeScenarios []nodeSetup) *testEnvironment {
	handlers := make(map[ids.NodeID]*BlockFetcherHandler[ExecutionBlock[container]])
	nodes := make([]ids.NodeID, len(nodeScenarios))

	for i, scenario := range nodeScenarios {
		nodeScenarios[i].nodeID = ids.GenerateTestNodeID()

		nodeID := nodeScenarios[i].nodeID
		nodes = append(nodes, nodeID)

		opts := make([]option, 3)
		opts = append(opts, withBlocks(scenario.blocks), withNodeID(nodeID))

		if scenario.blkControl != nil {
			opts = append(opts, withBlockControl(scenario.blkControl))
		}

		blkRetriever := newTestBlockRetriever(opts...)
		handlers[nodeID] = NewBlockFetcherHandler(blkRetriever)
	}

	return &testEnvironment{
		p2pBlockFetcher: &testP2PBlockFetcher{handlers: handlers, err: make(chan error, 1)},
		sampler:         newControlledNodeSampler(nodeScenarios),
		nodes:           nodes,
	}
}

func (t *testEnvironment) collectBlocksWithTimeout(ctx context.Context, resultChan <-chan ExecutionBlock[container], numRcvBlks int) (map[uint64]ExecutionBlock[container], error) {
	receivedBlocks := make(map[uint64]ExecutionBlock[container], numRcvBlks)
	done := make(chan struct{})

	innerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		defer close(done)
		for {
			select {
			case blk, ok := <-resultChan:
				if !ok {
					return
				}
				receivedBlocks[blk.GetHeight()] = blk
			case <-innerCtx.Done():
				return
			}
		}
	}()

	var err error
	select {
	case <-done:
		return receivedBlocks, nil
	case err = <-t.p2pBlockFetcher.err:
		cancel()
	case <-ctx.Done():
		cancel()
		err = ctx.Err()
	}

	<-done

	// We should allow partial responses
	return receivedBlocks, err
}

// === BlockParser ===
// parser implements BlockParser
type parser[T Block] struct {
	parseErr error
	state    map[ids.ID]T
}

// newParser is prefilling the parser state, mocking parsing
func newParser(chain []ExecutionBlock[container]) *parser[ExecutionBlock[container]] {
	p := &parser[ExecutionBlock[container]]{
		state: make(map[ids.ID]ExecutionBlock[container]),
	}

	for _, block := range chain {
		blockBytes := block.GetBytes()
		var blockID ids.ID
		copy(blockID[:], hashing.ComputeHash256(blockBytes))
		p.state[blockID] = block
	}

	return p
}

func (m *parser[T]) ParseBlock(_ context.Context, data []byte) (T, error) {
	if m.parseErr != nil {
		return utils.Zero[T](), m.parseErr
	}

	var blockID ids.ID
	hashedData := hashing.ComputeHash256(data)
	copy(blockID[:], hashedData)

	block, ok := m.state[blockID]
	if !ok {
		return utils.Zero[T](), fmt.Errorf("block %s not found", blockID)
	}
	return block, nil
}

// controlledNodeSampler provides deterministic node sampling for tests
// with explicit control over which node is sampled next
type controlledNodeSampler struct {
	nodesMap   map[uint8]ids.NodeID // Maps sampleOrder → nodeID
	orders     []uint8              // Sorted list of orders
	currentIdx int                  // Current index in orders slice
}

func newControlledNodeSampler(scenarios []nodeSetup) *controlledNodeSampler {
	if len(scenarios) == 0 {
		panic("scenarios cannot be empty")
	}

	// sort node scenarios by sampleOrder
	sort.Slice(scenarios, func(i, j int) bool {
		return scenarios[i].sampleOrder < scenarios[j].sampleOrder
	})

	nodesMap := make(map[uint8]ids.NodeID)
	orders := make([]uint8, 0, len(scenarios))

	// Collect the orders and node IDs
	for _, scenario := range scenarios {
		nodesMap[scenario.sampleOrder] = scenario.nodeID
		orders = append(orders, scenario.sampleOrder)
	}

	return &controlledNodeSampler{
		nodesMap: nodesMap,
		orders:   orders,
	}
}

func (s *controlledNodeSampler) Sample(_ context.Context, _ int) []ids.NodeID {
	order := s.orders[s.currentIdx]
	nodeID := s.nodesMap[order]

	s.currentIdx = (s.currentIdx + 1) % len(s.orders)
	return []ids.NodeID{nodeID}
}

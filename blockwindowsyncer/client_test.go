// Copyright (C) 2025, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blockwindowsyncer

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p/p2ptest"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/stretchr/testify/require"
)

// -----------------------------------------------------------------------------
// Mock implementations for testing
// -----------------------------------------------------------------------------

// mockBlock implements Block.
type mockBlock struct {
	height    uint64
	id        ids.ID
	parentID  ids.ID
	timestamp int64
	bytes     []byte
}

func newMockBlock(id ids.ID, parentID ids.ID, height uint64) *mockBlock {
	return &mockBlock{
		id:        id,
		parentID:  parentID,
		height:    height,
		timestamp: time.Now().UnixNano(),
		bytes:     id[:],
	}
}

func (m *mockBlock) GetID() ids.ID       { return m.id }
func (m *mockBlock) GetParent() ids.ID   { return m.parentID }
func (m *mockBlock) GetHeight() uint64   { return m.height }
func (m *mockBlock) GetTimestamp() int64 { return m.timestamp }
func (m *mockBlock) GetBytes() []byte    { return m.bytes }

func (m *mockBlock) String() string {
	return fmt.Sprintf("Block{id=%s, parent=%s, height=%d}", m.id, m.parentID, m.height)
}

// mockBlockParser implements BlockParser for *mockBlock.
type mockBlockParser struct {
	parseErr error
	writeErr error
	// blocks is a mapping from block.ID to the block.
	blocks map[ids.ID]*mockBlock
}

func (m *mockBlockParser) ParseBlock(_ context.Context, data []byte) (*mockBlock, error) {
	if m.parseErr != nil {
		return nil, m.parseErr
	}

	var blockID ids.ID
	copy(blockID[:], data)
	if blk, ok := m.blocks[blockID]; ok {
		return blk, nil
	}
	return nil, fmt.Errorf("block with id %s not found", blockID)
}

// WriteBlock “writes” the block. For our test we simply return any configured error.
func (m *mockBlockParser) WriteBlock(_ context.Context, _ *mockBlock) error {
	return m.writeErr
}

// mockNodeSampler implements p2p.NodeSampler.
type mockNodeSampler struct {
	nodes []ids.NodeID
}

func (m *mockNodeSampler) Sample(_ context.Context, num int) []ids.NodeID {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec
	r.Shuffle(len(m.nodes), func(i, j int) {
		m.nodes[i], m.nodes[j] = m.nodes[j], m.nodes[i]
	})
	if len(m.nodes) < num {
		return m.nodes
	}
	return m.nodes[:num]
}

// mockHandler implements p2p.Handler.
// It simulates a network peer that returns a BlockFetchResponse containing our chain.
type mockHandler struct {
	t             *testing.T
	responseData  [][]byte
	shouldError   bool
	delayResponse time.Duration
}

func (h *mockHandler) AppRequest(ctx context.Context, _ ids.NodeID, _ time.Time, _ []byte) ([]byte, *common.AppError) {
	if h.shouldError {
		return nil, &common.AppError{}
	}
	resp := &BlockFetchResponse{
		Blocks: h.responseData,
	}

	if h.delayResponse > 0 {
		timer := time.NewTimer(h.delayResponse)
		defer timer.Stop()
		select {
		case <-timer.C:
			return resp.MarshalCanoto(), nil
		case <-ctx.Done():
			return nil, &common.AppError{}
		}
	}
	return resp.MarshalCanoto(), nil
}

func (*mockHandler) AppGossip(_ context.Context, _ ids.NodeID, _ []byte) {}

// -----------------------------------------------------------------------------
// Tests
// -----------------------------------------------------------------------------

func TestBlockFetcherClient_Success(t *testing.T) {
	req := require.New(t)
	ctx := context.Background()

	// Create a chain of three blocks:
	// genesis -> block1 -> block2.
	genesisID := ids.GenerateTestID()
	block1ID := ids.GenerateTestID()
	block2ID := ids.GenerateTestID()

	genesis := newMockBlock(genesisID, ids.Empty, 0)
	block1 := newMockBlock(block1ID, genesisID, 1)
	block2 := newMockBlock(block2ID, block1ID, 2)

	// The chain we expect to fetch is [genesis, block1, block2] in order.
	blockChain := []*mockBlock{genesis, block1, block2}

	// Set up a parser with our chain blocks.
	parser := &mockBlockParser{
		blocks: map[ids.ID]*mockBlock{
			genesis.GetID(): genesis,
			block1.GetID():  block1,
			block2.GetID():  block2,
		},
	}

	// The handler returns the “serialized” blocks.
	responseData := make([][]byte, len(blockChain))
	for _, blk := range blockChain {
		responseData = append(responseData, blk.GetBytes())
	}
	handler := &mockHandler{
		t:            t,
		responseData: responseData,
	}

	serverNodeID := ids.GenerateTestNodeID()
	nodeSampler := &mockNodeSampler{
		nodes: []ids.NodeID{serverNodeID},
	}

	clientNodeID := ids.GenerateTestNodeID()
	client := p2ptest.NewClient(t, ctx, clientNodeID, handler, serverNodeID, handler)

	fetcher := NewBlockFetcherClient[*mockBlock](client, parser, nodeSampler)

	// Our target block is one whose parent is block2.
	targetBlockID := ids.GenerateTestID()
	targetBlock := newMockBlock(targetBlockID, block2ID, 3)

	err := fetcher.FetchBlock(ctx, targetBlock)
	req.NoError(err, "expected successful fetch")
}

func TestBlockFetcherClient_ParentVerificationFail(t *testing.T) {
	req := require.New(t)
	ctx := context.Background()

	genesisID := ids.GenerateTestID()
	block1ID := ids.GenerateTestID()
	block2ID := ids.GenerateTestID()
	block3ID := ids.GenerateTestID()

	genesis := newMockBlock(genesisID, ids.Empty, 0)
	block1 := newMockBlock(block1ID, genesisID, 1)
	block2 := newMockBlock(block2ID, block1ID, 2)

	// block3's parent should be block2, but we supply a different ID
	block3 := newMockBlock(block3ID, ids.GenerateTestID(), 3)

	targetBlock := newMockBlock(ids.GenerateTestID(), block3ID, 4)

	// The peer returned blockChain
	blockChain := []*mockBlock{genesis, block1, block2, block3}

	parser := &mockBlockParser{
		blocks: map[ids.ID]*mockBlock{
			genesis.GetID(): genesis,
			block1.GetID():  block1,
			block2.GetID():  block2,
			block3.GetID():  block3,
		},
	}

	responseData := make([][]byte, len(blockChain))
	for _, blk := range blockChain {
		responseData = append(responseData, blk.GetBytes())
	}
	handler := &mockHandler{
		t:            t,
		responseData: responseData,
	}

	serverNodeID := ids.GenerateTestNodeID()
	nodeSampler := &mockNodeSampler{
		nodes: []ids.NodeID{serverNodeID},
	}

	clientNodeID := ids.GenerateTestNodeID()
	client := p2ptest.NewClient(t, ctx, clientNodeID, handler, serverNodeID, handler)

	fetcher := NewBlockFetcherClient[*mockBlock](client, parser, nodeSampler)

	err := fetcher.FetchBlock(ctx, targetBlock)
	req.Error(err, "expected error due to parent verification failure")
	req.Contains(err.Error(), "block verification failed", "expected parent verification error")
}

func TestBlockFetcherClient_NoValidNode(t *testing.T) {
	req := require.New(t)
	ctx := context.Background()

	targetBlock := newMockBlock(ids.GenerateTestID(), ids.GenerateTestID(), 1)

	parser := &mockBlockParser{
		blocks: map[ids.ID]*mockBlock{},
	}

	nodeSampler := &mockNodeSampler{
		nodes: []ids.NodeID{},
	}

	handler := &mockHandler{
		t: t,
	}

	serverNodeID := ids.GenerateTestNodeID()
	clientNodeID := ids.GenerateTestNodeID()
	client := p2ptest.NewClient(t, ctx, clientNodeID, handler, serverNodeID, handler)

	fetcher := NewBlockFetcherClient[*mockBlock](client, parser, nodeSampler)

	err := fetcher.FetchBlock(ctx, targetBlock)
	req.Error(err, "expected error due to no valid nodes")
	req.Contains(err.Error(), "failed to sample valid nodeID", "expected error message about sampling")
}

func TestBlockFetcherClient_ContextCancelled(t *testing.T) {
	req := require.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	targetBlock := newMockBlock(ids.GenerateTestID(), ids.GenerateTestID(), 1)

	parser := &mockBlockParser{
		blocks: map[ids.ID]*mockBlock{},
	}

	serverNodeID := ids.GenerateTestNodeID()
	nodeSampler := &mockNodeSampler{
		nodes: []ids.NodeID{serverNodeID},
	}

	handler := &mockHandler{
		t: t,
	}

	clientNodeID := ids.GenerateTestNodeID()
	client := p2ptest.NewClient(t, ctx, clientNodeID, handler, serverNodeID, handler)

	fetcher := NewBlockFetcherClient[*mockBlock](client, parser, nodeSampler)

	err := fetcher.FetchBlock(ctx, targetBlock)
	req.Error(err, "expected error due to context cancellation")
	req.Contains(err.Error(), "context cancelled", "expected context cancelled error")
}

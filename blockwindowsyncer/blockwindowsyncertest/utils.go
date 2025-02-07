package blockwindowsyncertest

import (
	"context"
	"fmt"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"math/rand"
	"time"
)

var (
	_ p2p.NodeSampler = (*TestNodeSampler)(nil)
	_ p2p.Handler     = (*TestBlockFetcherHandler)(nil)
)

type testBlockRetriever interface {
	GetBlockByHeight(ctx context.Context, blockHeight uint64) (TestBlock, error)
}

type TestBlockFetcherHandler struct {
	retriever testBlockRetriever

	blockHeight  uint64
	minTimestamp int64
}

func (t TestBlockFetcherHandler) AppGossip(_ context.Context, _ ids.NodeID, _ []byte) {}

func (t TestBlockFetcherHandler) AppRequest(ctx context.Context, _ ids.NodeID, _ time.Time, requestBytes []byte) ([]byte, *common.AppError) {
	//TODO implement me
	panic("implement me")
}

type TestBlockRetriever struct {
	delay   time.Duration
	NodeID  ids.NodeID
	ErrChan chan error
	Blocks  map[uint64]*TestBlock
}

func NewTestBlockRetriever() *TestBlockRetriever {
	return &TestBlockRetriever{
		ErrChan: make(chan error, 1),
	}
}

func (r *TestBlockRetriever) WithBlocks(blocks map[uint64]*TestBlock) *TestBlockRetriever {
	r.Blocks = blocks
	return r
}

func (r *TestBlockRetriever) WithDelay(delay time.Duration) *TestBlockRetriever {
	r.delay = delay
	return r
}

func (r *TestBlockRetriever) WithNodeID(nodeID ids.NodeID) *TestBlockRetriever {
	r.NodeID = nodeID
	return r
}

func (r *TestBlockRetriever) GetBlockByHeight(_ context.Context, blockHeight uint64) (*TestBlock, error) {
	if r.delay.Nanoseconds() > 0 {
		time.Sleep(r.delay)
	}

	var err error
	if r.NodeID.Compare(ids.EmptyNodeID) == 0 {
		err = fmt.Errorf("block height %d not found", blockHeight)
	} else {
		err = fmt.Errorf("%s: block height %d not found", r.NodeID, blockHeight)
	}

	block, ok := r.Blocks[blockHeight]
	//fmt.Printf("GetBlockByHeight: node=%s height=%d exists=%v\n", r.NodeID, blockHeight, ok)

	if !ok && err != nil {
		r.ErrChan <- err
		return nil, err
	}
	return block, nil
}

// TestNodeSampler implements p2p.NodeSampler
type TestNodeSampler struct {
	Nodes []ids.NodeID
}

func (t *TestNodeSampler) Sample(_ context.Context, num int) []ids.NodeID {
	r := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec
	r.Shuffle(len(t.Nodes), func(i, j int) {
		t.Nodes[i], t.Nodes[j] = t.Nodes[j], t.Nodes[i]
	})
	if len(t.Nodes) < num {
		return t.Nodes
	}
	return t.Nodes[:num]
}

type TestBlock struct {
	height    uint64
	id        ids.ID
	parentID  ids.ID
	timestamp int64
	bytes     []byte
}

func NewTestBlock(id ids.ID, parentID ids.ID, height uint64, timestamp int64) *TestBlock {
	return &TestBlock{
		id:        id,
		parentID:  parentID,
		height:    height,
		timestamp: timestamp,
		bytes:     id[:],
	}
}

func (m *TestBlock) GetID() ids.ID       { return m.id }
func (m *TestBlock) GetParent() ids.ID   { return m.parentID }
func (m *TestBlock) GetHeight() uint64   { return m.height }
func (m *TestBlock) GetTimestamp() int64 { return m.timestamp }
func (m *TestBlock) GetBytes() []byte    { return m.bytes }
func (m *TestBlock) String() string {
	return fmt.Sprintf("Block{id=%s, parent=%s, height=%d}", m.id, m.parentID, m.height)
}

func GenerateChain(numBlocks int) []*TestBlock {
	if numBlocks <= 0 {
		return nil
	}

	chain := make([]*TestBlock, numBlocks)

	genesisID := ids.GenerateTestID()
	chain[0] = NewTestBlock(genesisID, ids.Empty, 0, 0)

	for i := 1; i < numBlocks; i++ {
		blockID := ids.GenerateTestID()
		parentID := chain[i-1].GetID()
		timestamp := chain[i-1].GetTimestamp()
		chain[i] = NewTestBlock(blockID, parentID, uint64(i), timestamp+1)
	}

	return chain
}

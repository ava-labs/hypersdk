package blockwindowsyncer

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/hypersdk/x/dsmr"
)

const (
	requestTimeout = 2 * time.Second // Timeout for each request
	numSampleNodes = 10              // Number of nodes to sample
)

var (
	errEmptyResponse = errors.New("empty response")
	errInvalidBlock  = errors.New("invalid block")
	errMaliciousNode = errors.New("malicious node")
)

// buffer is thread safe data structure for buffering
// pending blocks to be written (saved)

// todo: explain why buffer, what does it solve
type buffer[T Block] struct {
	mu      sync.Mutex
	pending map[ids.ID]T
}

func newBuffer[T Block]() *buffer[T] {
	return &buffer[T]{
		pending: make(map[ids.ID]T),
	}
}

func (b *buffer[T]) add(block T) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pending[block.GetID()] = block
}

func (b *buffer[T]) clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pending = make(map[ids.ID]T)
}

func (b *buffer[T]) getAll() []T {
	b.mu.Lock()
	defer b.mu.Unlock()

	blocks := make([]T, 0, len(b.pending))
	for _, blk := range b.pending {
		blocks = append(blocks, blk)
	}
	return blocks
}

type checkpoint struct {
	blockID   ids.ID
	height    uint64
	timestamp int64
}

type BlockFetcherClient[T Block] struct {
	client  *dsmr.TypedClient[*BlockFetchRequest, *BlockFetchResponse, []byte]
	parser  BlockParser[T]
	sampler p2p.NodeSampler

	buf       *buffer[T]
	writeChan chan struct{} // signals the writer goroutine to write buffered blocks
	errChan   chan error    // receives async errors (parsing/writing)

	checkpointL sync.RWMutex
	checkpoint  checkpoint

	// Stop/cleanup
	stopCh chan struct{}
	stopWG sync.WaitGroup
}

func NewBlockFetcherClient[T Block](
	baseClient *p2p.Client,
	parser BlockParser[T],
	sampler p2p.NodeSampler,
) *BlockFetcherClient[T] {
	c := &BlockFetcherClient[T]{
		client:  dsmr.NewTypedClient(baseClient, &blockFetcherMarshaler{}),
		parser:  parser,
		sampler: sampler,

		buf:       newBuffer[T](),
		writeChan: make(chan struct{}, 1),
		errChan:   make(chan error, 1),
		stopCh:    make(chan struct{}),
	}

	c.stopWG.Add(1)
	go func() {
		defer c.stopWG.Done()
		for {
			select {
			case <-c.stopCh:
				return
			case <-c.writeChan:
				c.writePendingBlocks(context.Background())
			}
		}
	}()

	return c
}

// Close stops the background goroutine. Call when shutting down.
func (c *BlockFetcherClient[T]) Close() error {
	close(c.stopCh)
	c.stopWG.Wait()
	return nil
}

func (c *BlockFetcherClient[T]) FetchBlocks(ctx context.Context, startBlock T, minTS int64) error {
	c.setCheckpoint(startBlock.GetID(), startBlock.GetHeight(), startBlock.GetTimestamp())
	req := &BlockFetchRequest{MinTimestamp: minTS}

	for {
		lastWritten := c.getCheckpoint()
		height := lastWritten.height
		lastWrittenBlockTS := lastWritten.timestamp

		if lastWrittenBlockTS <= minTS {
			break
		}

		nodeID := c.sampleNodeID(ctx)
		if nodeID.Compare(ids.EmptyNodeID) == 0 {
			// No node available
			time.Sleep(500 * time.Millisecond)
			continue
		}

		reqCtx, cancel := context.WithTimeout(ctx, requestTimeout)
		req.BlockHeight = height

		//fmt.Printf("sending request to %s blkHeight %d\n", nodeID, req)
		if err := c.client.AppRequest(reqCtx, nodeID, req, c.handleResponse); err != nil {
			cancel()
			// We'll retry with another node, so just continue
			continue
		}

		// Wait for parse/write error or context done
		select {
		case err := <-c.errChan:
			cancel()
			if errors.Is(err, context.Canceled) || errors.Is(err, errMaliciousNode) {
				return err
			}
		case <-ctx.Done():
			cancel()
			return ctx.Err()
		case <-reqCtx.Done():
			cancel()
		}

		// todo: update with backoff lib
		time.Sleep(200 * time.Millisecond)
	}
	return nil
}

func (c *BlockFetcherClient[T]) handleResponse(ctx context.Context, _ ids.NodeID, resp *BlockFetchResponse, reqErr error) {
	if reqErr != nil {
		c.errChan <- reqErr
		return
	}
	if len(resp.Blocks) == 0 {
		c.errChan <- errEmptyResponse
		return
	}

	c.checkpointL.RLock()
	lastWritten := c.checkpoint
	c.checkpointL.RUnlock()

	expectedBlockID := lastWritten.blockID
	for _, raw := range resp.Blocks {
		blk, err := c.parser.ParseBlock(ctx, raw)
		if err != nil {
			c.errChan <- fmt.Errorf("%w: %v", errInvalidBlock, err)
			return
		}

		if expectedBlockID != blk.GetID() {
			c.errChan <- errMaliciousNode
			goto write
		}
		expectedBlockID = blk.GetParent()
		c.buf.add(blk)
	}

write: // Trigger the background writer goroutine
	select {
	case c.writeChan <- struct{}{}:
	default:
	}
}

func (c *BlockFetcherClient[T]) writePendingBlocks(ctx context.Context) {
	blocks := c.buf.getAll()
	if len(blocks) == 0 {
		return
	}

	// Sort in descending order
	sort.Slice(blocks, func(i, j int) bool {
		return blocks[i].GetHeight() > blocks[j].GetHeight()
	})

	var lastBlock T
	for _, blk := range blocks {
		if err := c.parser.WriteBlock(ctx, blk); err != nil {
			c.errChan <- fmt.Errorf("failed to write block %s: %w", blk.GetID(), err)
			c.buf.clear()
			return
		}
		lastBlock = blk
	}

	if lastBlock.GetID().Compare(ids.Empty) != 0 {
		// Since we have the last block saved and sequence matters (N, N-1, N-2, ... N-K), next block height to fetch is one below it
		c.setCheckpoint(lastBlock.GetParent(), lastBlock.GetHeight()-1, lastBlock.GetTimestamp())
	}

	// Blocks have been written successfully
	c.buf.clear()
}

// sampleNodeID picks a random node from the node sampler.
func (c *BlockFetcherClient[T]) sampleNodeID(ctx context.Context) ids.NodeID {
	nodes := c.sampler.Sample(ctx, numSampleNodes)
	for _, nodeID := range nodes {
		if nodeID != ids.EmptyNodeID {
			return nodeID
		}
	}
	return ids.EmptyNodeID
}

func (c *BlockFetcherClient[T]) setCheckpoint(blockID ids.ID, height uint64, ts int64) {
	c.checkpointL.Lock()
	defer c.checkpointL.Unlock()
	c.checkpoint.blockID = blockID
	c.checkpoint.height = height
	c.checkpoint.timestamp = ts
}

func (c *BlockFetcherClient[T]) getCheckpoint() checkpoint {
	c.checkpointL.RLock()
	defer c.checkpointL.RUnlock()
	return c.checkpoint
}

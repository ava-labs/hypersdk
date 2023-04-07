package vm

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/heap"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/neilotoole/errgroup"
	"go.uber.org/zap"
)

// TODO: make max retries and failure sleep configurable
const (
	maxChunkRetries = 20
	retrySleep      = 100 * time.Millisecond
	gossipFrequency = 100 * time.Millisecond
)

type NodeChunks struct {
	Min         uint64
	Max         uint64
	Unprocessed set.Set[ids.ID]
}

func (n *NodeChunks) Marshal() ([]byte, error) {
	p := codec.NewWriter(consts.NetworkSizeLimit)
	p.PackUint64(n.Min)
	p.PackUint64(n.Max)
	l := len(n.Unprocessed)
	p.PackInt(l)
	for chunk := range n.Unprocessed {
		p.PackID(chunk)
	}
	return p.Bytes(), p.Err()
}

func UnmarshalNodeChunks(b []byte) (*NodeChunks, error) {
	var n NodeChunks
	p := codec.NewReader(b, consts.NetworkSizeLimit)
	n.Min = p.UnpackUint64(false) // could be genesis
	n.Max = p.UnpackUint64(false) // could be genesis
	l := p.UnpackInt(false)       // could have no processing
	n.Unprocessed = set.NewSet[ids.ID](l)
	for i := 0; i < l; i++ {
		var chunk ids.ID
		p.UnpackID(true, &chunk)
		n.Unprocessed.Add(chunk)
	}
	return &n, p.Err()
}

type bucket struct {
	h     uint64   // Timestamp
	items []ids.ID // Array of AvalancheGo ids
}

type ChunkMap struct {
	bh      *heap.Heap[*bucket, uint64]
	counts  map[ids.ID]int
	heights map[uint64]*bucket // Uses timestamp as keys to map to buckets of ids.
}

func NewChunkMap() *ChunkMap {
	// If lower height is accepted and chunk in rejected block that shows later,
	// must not remove yet.
	return &ChunkMap{
		counts:  map[ids.ID]int{},
		heights: make(map[uint64]*bucket),
		bh:      heap.New[*bucket, uint64](120, true),
	}
}

func (c *ChunkMap) Add(height uint64, chunkID ids.ID) {
	// Increase chunk count
	times := c.counts[chunkID]
	c.counts[chunkID] = times + 1

	// Check if bucket with height already exists
	if b, ok := c.heights[height]; ok {
		b.items = append(b.items, chunkID)
		return
	}

	// Create new bucket
	b := &bucket{
		h:     height,
		items: []ids.ID{chunkID},
	}
	c.heights[height] = b
	c.bh.Push(&heap.Entry[*bucket, uint64]{
		ID:    chunkID,
		Val:   height,
		Item:  b,
		Index: c.bh.Len(),
	})
}

func (c *ChunkMap) SetMin(h uint64) []ids.ID {
	evicted := []ids.ID{}
	for {
		b := c.bh.First()
		if b == nil || b.Val >= h {
			break
		}
		c.bh.Pop()
		for _, id := range b.Item.items {
			count := c.counts[id]
			count--
			if count == 0 {
				delete(c.counts, id)
				evicted = append(evicted, id)
			} else {
				c.counts[id] = count
			}
		}
		// Delete from times map
		delete(c.heights, b.Val)
	}
	return evicted
}

func (c *ChunkMap) All() set.Set[ids.ID] {
	s := set.NewSet[ids.ID](len(c.counts))
	for k := range c.counts {
		s.Add(k)
	}
	return s
}

type ChunkManager struct {
	vm        *VM
	appSender common.AppSender

	l         sync.Mutex
	requestID uint32
	requests  map[uint32]chan []byte

	cl            sync.RWMutex
	fetchedChunks map[ids.ID][]byte
	chunks        *ChunkMap

	ml sync.RWMutex
	m  map[ids.NodeID]*NodeChunks

	min         uint64
	max         uint64
	lastChanged time.Time
	sl          sync.Mutex

	done chan struct{}
}

func NewChunkManager(vm *VM) *ChunkManager {
	return &ChunkManager{
		vm:            vm,
		requests:      map[uint32]chan []byte{},
		fetchedChunks: map[ids.ID][]byte{},
		chunks:        NewChunkMap(),
		m:             map[ids.NodeID]*NodeChunks{},
		done:          make(chan struct{}),
	}
}

func (c *ChunkManager) Run(appSender common.AppSender) {
	c.appSender = appSender

	c.vm.Logger().Info("starting chunk manager")
	defer close(c.done)

	t := time.NewTicker(gossipFrequency)
	lastSent := time.Now()
	defer t.Stop()
	for {
		select {
		case <-t.C:
			c.sl.Lock()
			if c.lastChanged.Sub(lastSent) < 0 {
				c.sl.Unlock()
				continue
			}
			nc := &NodeChunks{
				Min:         c.min,
				Max:         c.max,
				Unprocessed: c.chunks.All(),
			}
			b, err := nc.Marshal()
			c.sl.Unlock()
			if err != nil {
				c.vm.snowCtx.Log.Warn("unable to marshal chunk gossip", zap.Error(err))
				continue
			}
			if err := c.appSender.SendAppGossip(context.TODO(), b); err != nil {
				c.vm.snowCtx.Log.Warn("unable to send chunk gossip", zap.Error(err))
				continue
			}
			lastSent = time.Now()
		case <-c.vm.stop:
			c.vm.Logger().Info("stopping chunk manager")
			return
		}
	}
}

// Called when building a chunk
func (c *ChunkManager) RegisterChunk(ctx context.Context, height uint64, chunk []byte) {
	chunkID := utils.ToID(chunk)
	c.cl.Lock()
	c.fetchedChunks[chunkID] = chunk
	c.chunks.Add(height, chunkID)
	c.cl.Unlock()
}

// Called when pruning chunks from accepted blocks
//
// Chunks should be pruned AFTER this is called
// TODO: Set when pruning blobs
// TODO: Set when state syncing
func (c *ChunkManager) SetMin(min uint64) {
	c.sl.Lock()
	c.min = min
	c.lastChanged = time.Now()
	c.sl.Unlock()
}

// Called when a block is accepted
//
// Ensure chunks are persisted before calling this method
func (c *ChunkManager) Accept(height uint64) {
	c.sl.Lock()
	c.max = height
	c.lastChanged = time.Now()
	for _, chunkID := range c.chunks.SetMin(height + 1) {
		delete(c.fetchedChunks, chunkID)
	}
	c.sl.Unlock()
}

func (c *ChunkManager) RequestChunks(ctx context.Context, height uint64, chunkIDs []ids.ID, ch chan []byte) error {
	start := time.Now()
	defer c.vm.metrics.chunksFetched.Observe(float64(time.Since(start)))

	// TODO: de-deuplicate requests for same chunk
	// TODO: pre-store chunks on disk if bootstrapping
	g, gctx := errgroup.WithContext(ctx)
	for _, cchunkID := range chunkIDs {
		chunkID := cchunkID
		g.Go(func() error {
			return c.requestChunk(gctx, height, chunkID, ch)
		})
	}
	if err := g.Wait(); err != nil {
		c.vm.metrics.chunkJobFails.Inc()
		return err
	}
	return nil
}

// requestChunk attempts to fetch a chunk and sends it on [ch] when it does, or
// returns an error.
func (c *ChunkManager) requestChunk(ctx context.Context, height uint64, chunkID ids.ID, ch chan []byte) error {
	c.vm.metrics.chunkRequests.Inc()

	c.cl.RLock()
	chunk, ok := c.fetchedChunks[chunkID]
	c.cl.RUnlock()
	if ok {
		ch <- chunk
		return nil
	}

	for i := 0; i < maxChunkRetries; i++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		// Determine who to send request to
		possibleRecipients := []ids.NodeID{}
		c.ml.RLock()
		for nodeID, chunk := range c.m {
			if height >= chunk.Min && height <= chunk.Max {
				possibleRecipients = append(possibleRecipients, nodeID)
				continue
			}
			if chunk.Unprocessed.Contains(chunkID) {
				possibleRecipients = append(possibleRecipients, nodeID)
			}
		}
		c.ml.RUnlock()
		if len(possibleRecipients) == 0 {
			c.vm.snowCtx.Log.Warn("no possible recipients", zap.Stringer("chunkID", chunkID))
			time.Sleep(retrySleep)
			continue
		}
		recipient := possibleRecipients[rand.Intn(len(possibleRecipients))]

		// Send request
		rch := make(chan []byte)
		c.l.Lock()
		requestID := c.requestID
		c.requestID++
		c.requests[requestID] = rch
		c.l.Unlock()
		if err := c.appSender.SendAppRequest(
			ctx,
			set.Set[ids.NodeID]{recipient: struct{}{}},
			requestID,
			chunkID[:],
		); err != nil {
			return err
		}

		// Handle request
		var msg []byte
		select {
		case msg = <-rch:
		case <-ctx.Done():
			return ctx.Err()
		}
		if len(msg) == 0 {
			c.vm.metrics.failedChunkRequests.Inc()
			c.vm.snowCtx.Log.Warn("chunk fetch failed", zap.Stringer("chunkID", chunkID))
			time.Sleep(retrySleep)
			continue
		}
		fchunkID := utils.ToID(msg)
		if chunkID != fchunkID {
			// TODO: penalize sender
			c.vm.metrics.failedChunkRequests.Inc()
			c.vm.snowCtx.Log.Warn("received incorrect chunk", zap.Stringer("nodeID", recipient))
			time.Sleep(retrySleep)
			continue
		}
		c.cl.Lock()
		c.fetchedChunks[chunkID] = msg
		c.chunks.Add(height, chunkID)
		c.cl.Unlock()
		ch <- msg
		return nil
	}
	return errors.New("could not fetch chunk")
}

func (c *ChunkManager) HandleRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	request []byte,
) error {
	chunkID, err := ids.ToID(request)
	if err != nil {
		c.vm.snowCtx.Log.Warn("unable to parse chunk request", zap.Error(err))
		return nil
	}

	// Check processing
	c.cl.RLock()
	chunk, ok := c.fetchedChunks[chunkID]
	c.cl.RUnlock()
	if ok {
		return c.appSender.SendAppResponse(ctx, nodeID, requestID, chunk)
	}

	// Check accepted
	chunk, err = c.vm.GetChunk(chunkID)
	if err != nil {
		c.vm.snowCtx.Log.Warn("unable to find chunk", zap.Error(err))
		return c.appSender.SendAppResponse(ctx, nodeID, requestID, []byte{})
	}
	return c.appSender.SendAppResponse(ctx, nodeID, requestID, chunk)
}

func (c *ChunkManager) HandleResponse(nodeID ids.NodeID, requestID uint32, msg []byte) error {
	c.l.Lock()
	request, ok := c.requests[requestID]
	if !ok {
		c.l.Unlock()
		return nil
	}
	delete(c.requests, requestID)
	c.l.Unlock()
	request <- msg
	return nil
}

func (c *ChunkManager) HandleRequestFailed(requestID uint32) error {
	c.l.Lock()
	request, ok := c.requests[requestID]
	if !ok {
		c.l.Unlock()
		return nil
	}
	delete(c.requests, requestID)
	c.l.Unlock()
	request <- []byte{}
	return nil
}

func (c *ChunkManager) HandleAppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	nc, err := UnmarshalNodeChunks(msg)
	if err != nil {
		c.vm.Logger().Error("unable to parse chunk gossip", zap.Error(err))
		return nil
	}
	// TODO: optimistically fetch new processing chunks in case we need them
	c.ml.Lock()
	c.m[nodeID] = nc
	c.ml.Unlock()
	return nil
}

func (c *ChunkManager) Done() {
	<-c.done
}

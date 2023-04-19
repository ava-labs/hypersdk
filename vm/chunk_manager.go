package vm

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/cache"
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
	retrySleep      = 50 * time.Millisecond
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
	h     uint64          // Timestamp
	items set.Set[ids.ID] // Array of AvalancheGo ids
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
	// Ensure chunk is not already registered at height
	b, ok := c.heights[height]
	if ok && b.items.Contains(chunkID) {
		return
	}

	// Increase chunk count
	times := c.counts[chunkID]
	c.counts[chunkID] = times + 1

	// Check if bucket with height already exists
	if ok {
		b.items.Add(chunkID)
		return
	}

	// Create new bucket
	b = &bucket{
		h:     height,
		items: set.Set[ids.ID]{chunkID: struct{}{}},
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
		for chunkID := range b.Item.items {
			count := c.counts[chunkID]
			count--
			if count == 0 {
				delete(c.counts, chunkID)
				evicted = append(evicted, chunkID)
			} else {
				c.counts[chunkID] = count
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

	requestLock sync.Mutex
	requestID   uint32
	requests    map[uint32]chan []byte

	chunkLock        sync.RWMutex
	fetchedChunks    map[ids.ID][]byte
	optimisticChunks *cache.LRU[ids.ID, []byte]
	clearedChunks    *cache.LRU[ids.ID, struct{}]

	chunks *ChunkMap
	min    uint64
	max    uint64

	nodeChunkLock sync.RWMutex
	nodeChunks    map[ids.NodeID]*NodeChunks
	nodeSet       set.Set[ids.NodeID]

	outstandingLock sync.Mutex
	outstanding     map[ids.ID][]chan *chunkResult

	update chan struct{}
	done   chan struct{}
}

func NewChunkManager(vm *VM) *ChunkManager {
	return &ChunkManager{
		vm:               vm,
		requests:         map[uint32]chan []byte{},
		fetchedChunks:    map[ids.ID][]byte{},
		optimisticChunks: &cache.LRU[ids.ID, []byte]{Size: 1024},
		clearedChunks:    &cache.LRU[ids.ID, struct{}]{Size: 1024},
		chunks:           NewChunkMap(),
		nodeChunks:       map[ids.NodeID]*NodeChunks{},
		nodeSet:          set.NewSet[ids.NodeID](64),
		outstanding:      map[ids.ID][]chan *chunkResult{},
		update:           make(chan struct{}),
		done:             make(chan struct{}),
	}
}

func (c *ChunkManager) Run(appSender common.AppSender) {
	c.appSender = appSender

	c.vm.Logger().Info("starting chunk manager")
	defer close(c.done)

	timer := time.NewTicker(gossipFrequency)
	defer timer.Stop()

	for {
		select {
		case <-c.update:
		case <-timer.C:
			// TODO: consider removing timer if we are already sending to everyone
		case <-c.vm.stop:
			c.vm.Logger().Info("stopping chunk manager")
			return
		}

		// TODO: advertise optimistic chunks
		c.chunkLock.RLock()
		nc := &NodeChunks{
			Min:         c.min,
			Max:         c.max,
			Unprocessed: c.chunks.All(),
		}
		c.chunkLock.RUnlock() // chunks is copied
		b, err := nc.Marshal()
		if err != nil {
			c.vm.snowCtx.Log.Warn("unable to marshal chunk gossip", zap.Error(err))
			continue
		}
		// We attempt to gossip the chunks we have to everyone
		if err := c.appSender.SendAppGossipSpecific(context.TODO(), c.nodeSet, b); err != nil {
			c.vm.snowCtx.Log.Warn("unable to send chunk gossip", zap.Error(err))
			continue
		}
		c.vm.metrics.chunksProcessing.Set(float64(len(nc.Unprocessed)))
	}
}

// Called when building a chunk
func (c *ChunkManager) RegisterChunks(ctx context.Context, height uint64, chunks [][]byte) {
	chunkIDs := make([]ids.ID, len(chunks))
	for i, chunk := range chunks {
		chunkIDs[i] = utils.ToID(chunk)
	}
	c.chunkLock.Lock()
	for i, chunk := range chunks {
		c.fetchedChunks[chunkIDs[i]] = chunk
		c.chunks.Add(height, chunkIDs[i])
	}
	c.chunkLock.Unlock()

	c.update <- struct{}{}
}

// Called when pruning chunks from accepted blocks
//
// Chunks should be pruned AFTER this is called
// TODO: Set when pruning blobs
// TODO: Set when state syncing
func (c *ChunkManager) SetMin(min uint64) {
	c.chunkLock.Lock()
	c.min = min
	c.chunkLock.Unlock()

	c.update <- struct{}{}
}

// Called when a block is accepted
//
// Ensure chunks are persisted before calling this method
func (c *ChunkManager) Accept(height uint64) {
	c.chunkLock.Lock()
	c.max = height
	evicted := c.chunks.SetMin(height + 1)
	for _, chunkID := range evicted {
		delete(c.fetchedChunks, chunkID)
		c.clearedChunks.Put(chunkID, struct{}{})
		c.optimisticChunks.Evict(chunkID)
	}
	processing := len(c.fetchedChunks)
	c.chunkLock.Unlock()

	c.update <- struct{}{}
	c.vm.snowCtx.Log.Info("evicted chunks from memory", zap.Int("n", len(evicted)), zap.Int("processing", processing))
}

func (c *ChunkManager) RequestChunks(ctx context.Context, height uint64, chunkIDs []ids.ID, ch chan []byte) error {
	start := time.Now()
	defer c.vm.metrics.chunksFetched.Observe(float64(time.Since(start)))

	// TODO: pre-store chunks on disk if bootstrapping
	g, gctx := errgroup.WithContext(ctx)
	for _, cchunkID := range chunkIDs {
		chunkID := cchunkID
		g.Go(func() error {
			crch := make(chan *chunkResult, 1)
			c.RequestChunk(gctx, &height, ids.EmptyNodeID, chunkID, crch)
			select {
			case r := <-crch:
				if r.err != nil {
					return r.err
				}
				ch <- r.chunk
				return nil
			case <-gctx.Done():
				return gctx.Err()
			}
		})
	}
	if err := g.Wait(); err != nil {
		c.vm.metrics.chunkJobFails.Inc()
		return err
	}

	// Trigger that we have processed new chunks
	c.update <- struct{}{}
	return nil
}

type chunkResult struct {
	chunk []byte
	err   error
}

func (c *ChunkManager) sendToOutstandingListeners(chunkID ids.ID, chunk []byte, err error) {
	c.outstandingLock.Lock()
	listeners, ok := c.outstanding[chunkID]
	delete(c.outstanding, chunkID)
	c.outstandingLock.Unlock()
	if !ok {
		return
	}
	result := &chunkResult{chunk, err}
	for _, listener := range listeners {
		if listener == nil {
			continue
		}
		listener <- result
	}
}

// RequestChunk may spawn a goroutine
func (c *ChunkManager) RequestChunk(ctx context.Context, height *uint64, hint ids.NodeID, chunkID ids.ID, ch chan *chunkResult) {
	// Register request to be notified
	c.outstandingLock.Lock()
	outstanding, ok := c.outstanding[chunkID]
	if ok {
		c.outstanding[chunkID] = append(outstanding, ch)
	} else {
		c.outstanding[chunkID] = []chan *chunkResult{ch}
	}
	c.outstandingLock.Unlock()
	if ok {
		// Wait for requests to eventually return
		return
	}

	// Check if previously fetched
	c.chunkLock.Lock()
	if chunk, ok := c.fetchedChunks[chunkID]; ok {
		if height != nil {
			c.chunks.Add(*height, chunkID)
		}
		c.chunkLock.Unlock()
		c.sendToOutstandingListeners(chunkID, chunk, nil)
		return
	}
	c.chunkLock.Unlock()

	// Check if optimistically cached
	if chunk, ok := c.optimisticChunks.Get(chunkID); ok {
		c.chunkLock.Lock()
		if height != nil {
			c.fetchedChunks[chunkID] = chunk
			c.chunks.Add(*height, chunkID)
		}
		c.chunkLock.Unlock()
		c.sendToOutstandingListeners(chunkID, chunk, nil)
		return
	}

	// Attempt to fetch
	for i := 0; i < maxChunkRetries; i++ {
		if err := ctx.Err(); err != nil {
			c.sendToOutstandingListeners(chunkID, nil, err)
			return
		}

		var peer ids.NodeID
		if hint != ids.EmptyNodeID && i <= 1 {
			peer = hint
		} else {
			// Determine who to send request to
			possibleRecipients := []ids.NodeID{}
			var randomRecipient ids.NodeID
			c.nodeChunkLock.RLock()
			for nodeID, chunk := range c.nodeChunks {
				randomRecipient = nodeID
				if height != nil && *height >= chunk.Min && *height <= chunk.Max {
					possibleRecipients = append(possibleRecipients, nodeID)
					continue
				}
				if chunk.Unprocessed.Contains(chunkID) {
					possibleRecipients = append(possibleRecipients, nodeID)
					continue
				}
			}
			c.nodeChunkLock.RUnlock()

			// No possible recipients, so we wait
			if randomRecipient == ids.EmptyNodeID {
				time.Sleep(retrySleep)
				continue
			}

			// If 1 or more possible recipients, pick them instead
			if len(possibleRecipients) > 0 {
				randomRecipient = possibleRecipients[rand.Intn(len(possibleRecipients))]
			} else {
				if height == nil {
					c.vm.snowCtx.Log.Warn("no possible recipients", zap.Stringer("chunkID", chunkID), zap.Stringer("hint", hint))
				} else {
					c.vm.snowCtx.Log.Warn("no possible recipients", zap.Stringer("chunkID", chunkID), zap.Stringer("hint", hint), zap.Uint64("height", *height))
				}
			}
			peer = randomRecipient
		}

		// Handle received message
		msg, err := c.requestChunkNodeID(ctx, peer, chunkID)
		if err != nil {
			time.Sleep(retrySleep)
			continue
		}
		if height != nil {
			c.chunkLock.Lock()
			c.fetchedChunks[chunkID] = msg
			c.chunks.Add(*height, chunkID)
			c.chunkLock.Unlock()
		} else {
			c.vm.snowCtx.Log.Info("optimistically fetched chunk", zap.Stringer("chunkID", chunkID), zap.Int("size", len(msg)))
			c.optimisticChunks.Put(chunkID, msg)
		}
		c.sendToOutstandingListeners(chunkID, msg, nil)
		return
	}
	c.sendToOutstandingListeners(chunkID, nil, errors.New("exhausted retries"))
}

func (c *ChunkManager) requestChunkNodeID(ctx context.Context, recipient ids.NodeID, chunkID ids.ID) ([]byte, error) {
	c.vm.metrics.chunkRequests.Inc()

	// Send request
	rch := make(chan []byte)
	c.requestLock.Lock()
	requestID := c.requestID
	c.requestID++
	c.requests[requestID] = rch
	c.requestLock.Unlock()
	if err := c.appSender.SendAppRequest(
		ctx,
		set.Set[ids.NodeID]{recipient: struct{}{}},
		requestID,
		chunkID[:],
	); err != nil {
		c.vm.metrics.failedChunkRequests.Inc()
		c.vm.snowCtx.Log.Warn("chunk fetch request failed", zap.Stringer("chunkID", chunkID), zap.Error(err))
		return nil, err
	}

	// Handle request
	var msg []byte
	select {
	case msg = <-rch:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if len(msg) == 0 {
		c.vm.metrics.failedChunkRequests.Inc()
		// Happens if recipient does not have the chunk we want
		c.vm.snowCtx.Log.Warn("chunk fetch returned empty", zap.Stringer("chunkID", chunkID))
		return nil, errors.New("not found")
	}
	fchunkID := utils.ToID(msg)
	if chunkID != fchunkID {
		// TODO: penalize sender
		c.vm.metrics.failedChunkRequests.Inc()
		c.vm.snowCtx.Log.Warn("received incorrect chunk", zap.Stringer("nodeID", recipient))
		return nil, errors.New("invalid chunk")
	}
	return msg, nil
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
	c.chunkLock.RLock()
	chunk, ok := c.fetchedChunks[chunkID]
	c.chunkLock.RUnlock()
	if ok {
		return c.appSender.SendAppResponse(ctx, nodeID, requestID, chunk)
	}

	// Check accepted
	chunk, err = c.vm.GetChunk(chunkID)
	if err != nil {
		c.vm.snowCtx.Log.Warn("unable to find chunk", zap.Stringer("chunkID", chunkID), zap.Error(err))
		return c.appSender.SendAppResponse(ctx, nodeID, requestID, []byte{})
	}
	return c.appSender.SendAppResponse(ctx, nodeID, requestID, chunk)
}

func (c *ChunkManager) HandleResponse(nodeID ids.NodeID, requestID uint32, msg []byte) error {
	c.requestLock.Lock()
	request, ok := c.requests[requestID]
	if !ok {
		c.requestLock.Unlock()
		c.vm.snowCtx.Log.Warn("got unexpected response", zap.Uint32("requestID", requestID))
		return nil
	}
	delete(c.requests, requestID)
	c.requestLock.Unlock()
	request <- msg
	return nil
}

func (c *ChunkManager) HandleRequestFailed(requestID uint32) error {
	c.requestLock.Lock()
	request, ok := c.requests[requestID]
	if !ok {
		c.requestLock.Unlock()
		c.vm.snowCtx.Log.Warn("unexpected request failed", zap.Uint32("requestID", requestID))
		return nil
	}
	delete(c.requests, requestID)
	c.requestLock.Unlock()
	request <- []byte{}
	return nil
}

func (c *ChunkManager) HandleAppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	nc, err := UnmarshalNodeChunks(msg)
	if err != nil {
		c.vm.Logger().Error("unable to parse chunk gossip", zap.Error(err))
		return nil
	}
	c.nodeChunkLock.Lock()
	c.nodeChunks[nodeID] = nc
	unprocessed := nc.Unprocessed // never updated
	c.nodeChunkLock.Unlock()

	// Optimistically fetch chunks
	// TODO: only fetch if from a soon to be producer (i.e. will need to verify
	// a future block)
	// TODO: handle case where already wrote to disk and we are getting old
	// chunks
	for chunkID := range unprocessed {
		if _, ok := c.clearedChunks.Get(chunkID); ok {
			continue
		}
		// TODO: limit max concurrency here
		go c.RequestChunk(context.Background(), nil, nodeID, chunkID, nil)
	}
	return nil
}

// Send info to new peer on handshake
func (c *ChunkManager) HandleConnect(ctx context.Context, nodeID ids.NodeID) error {
	c.chunkLock.RLock()
	nc := &NodeChunks{
		Min:         c.min,
		Max:         c.max,
		Unprocessed: c.chunks.All(),
	}
	c.chunkLock.RUnlock() // chunks is copied
	b, err := nc.Marshal()
	if err != nil {
		c.vm.snowCtx.Log.Warn("unable to marshal chunk gossip specific ", zap.Error(err))
		return nil
	}
	if err := c.appSender.SendAppGossipSpecific(context.TODO(), set.Set[ids.NodeID]{nodeID: struct{}{}}, b); err != nil {
		c.vm.snowCtx.Log.Warn("unable to send chunk gossip", zap.Error(err))
		return nil
	}
	c.nodeChunkLock.Lock()
	c.nodeSet.Add(nodeID)
	c.nodeChunkLock.Unlock()
	return nil
}

// When disconnecting from a node, we remove it from the map because we should
// no longer request chunks from it.
func (c *ChunkManager) HandleDisconnect(ctx context.Context, nodeID ids.NodeID) error {
	c.nodeChunkLock.Lock()
	delete(c.nodeChunks, nodeID)
	c.nodeSet.Remove(nodeID)
	c.nodeChunkLock.Unlock()
	return nil
}

func (c *ChunkManager) Done() {
	<-c.done
}

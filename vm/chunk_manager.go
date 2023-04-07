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

// TODO: gossip chunks as soon as build block (before verify)
// TODO: automatically fetch new chunks before needed (will better control
// which we fetch in the future)
// TODO: allow for deleting block chunks after some period of time
type NodeChunks struct {
	Min         uint64
	Max         uint64
	Unprocessed set.Set[ids.ID]
}

func (n *NodeChunks) Marshal() ([]byte, error) {
	// TODO: set to network limit
	p := codec.NewWriter(consts.MaxInt)
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
	p := codec.NewReader(b, consts.MaxInt)
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
	mu sync.RWMutex

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
	c.mu.Lock()
	defer c.mu.Unlock()

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
	c.mu.Lock()
	defer c.mu.Unlock()

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
	c.mu.RLock()
	defer c.mu.RUnlock()

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

	// TODO: need to determine when to write to disk/clear cache
	// May not end up verifying a block we request on
	// TODO: make sure to pin chunks to block in case they are cleared from LRU
	// during verification
	// TODO: store []byte + height -> discard lazily if fetched chunks are below
	// accepted
	cl            sync.RWMutex
	fetchedChunks map[ids.ID][]byte
	// Duplicate chunks at different heights (only first added will be included)
	chunks *ChunkMap

	m  map[ids.NodeID]*NodeChunks
	ml sync.RWMutex

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

	t := time.NewTicker(100 * time.Millisecond)
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
			// TODO: need to iterate over LRU to get all hashes
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
// TODO: set on initialization based on what is in store
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
	// TODO: de-deuplicate requests for same chunk
	g, gctx := errgroup.WithContext(ctx)
	for _, cchunkID := range chunkIDs {
		chunkID := cchunkID
		g.Go(func() error {
			return c.requestChunk(gctx, height, chunkID, ch)
		})
	}
	return g.Wait()
}

// TODO: register a request chunks job instead of going one-by-one
// Can then parse and add to the block async instead of doing during verify
// Run signature verification job per blob (wait at the end)
// Make X attempts and then abandon (can be retrigged by future verify job)
// If state isn't ready, just put job in failure state...will fetch if needed
// during verify
func (c *ChunkManager) requestChunk(ctx context.Context, height uint64, chunkID ids.ID, ch chan []byte) error {
	c.cl.RLock()
	if chunk, ok := c.fetchedChunks[chunkID]; ok {
		c.cl.RUnlock()
		ch <- chunk
		return nil
	}
	c.cl.RUnlock()

	for i := 0; i < 5; i++ {
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
			time.Sleep(500 * time.Millisecond)
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
		msg := <-rch
		if len(msg) == 0 {
			c.vm.snowCtx.Log.Warn("chunk fetch failed", zap.Stringer("chunkID", chunkID))
			time.Sleep(500 * time.Millisecond)
			continue
		}
		fchunkID := utils.ToID(msg)
		if chunkID != fchunkID {
			c.vm.snowCtx.Log.Warn("received incorrect chunk", zap.Stringer("nodeID", recipient))
			// TODO: penalize sender
			time.Sleep(500 * time.Millisecond)
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
	c.cl.RLock()
	if chunk, ok := c.fetchedChunks[chunkID]; ok {
		c.cl.RUnlock()
		return c.appSender.SendAppResponse(ctx, nodeID, requestID, chunk)
	}
	c.cl.RUnlock()
	chunk, err := c.vm.GetChunk(chunkID)
	if err != nil {
		c.vm.snowCtx.Log.Warn("unable to fetch chunk", zap.Error(err))
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
	c.ml.Lock()
	c.m[nodeID] = nc
	c.ml.Unlock()
	return nil
}

func (c *ChunkManager) Done() {
	<-c.done
}

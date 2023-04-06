package vm

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/utils"
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

type ChunkManager struct {
	vm        *VM
	appSender common.AppSender

	l           sync.Mutex
	requestID   uint32
	outstanding set.Set[ids.ID]
	requests    map[uint32]ids.ID

	// TODO: need to determine when to write to disk/clear cache
	// May not end up verifying a block we request on
	// TODO: make sure to pin chunks to block in case they are cleared from LRU
	// during verification
	fetchedChunks *cache.LRU[ids.ID, []byte]

	m  map[ids.NodeID]*NodeChunks
	ml sync.RWMutex

	min         uint64
	max         uint64
	processing  set.Set[ids.ID]
	lastChanged time.Time
	sl          sync.Mutex

	done chan struct{}
}

func NewChunkManager(vm *VM) *ChunkManager {
	return &ChunkManager{
		vm:            vm,
		outstanding:   set.NewSet[ids.ID](128),
		requests:      map[uint32]ids.ID{},
		fetchedChunks: &cache.LRU[ids.ID, []byte]{Size: 1024},
		m:             map[ids.NodeID]*NodeChunks{},
		processing:    set.NewSet[ids.ID](128),
		done:          make(chan struct{}),
	}
}

func (c *ChunkManager) Run(ctx context.Context, appSender common.AppSender) {
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
			nc := &NodeChunks{
				Min:         c.min,
				Max:         c.max,
				Unprocessed: c.processing,
			}
			b, err := nc.Marshal()
			c.sl.Unlock()
			if err != nil {
				c.vm.snowCtx.Log.Warn("unable to marshal chunk gossip", zap.Error(err))
				continue
			}
			if err := c.appSender.SendAppGossip(ctx, b); err != nil {
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
func (c *ChunkManager) RegisterChunk(ctx context.Context, chunkID ids.ID, chunk []byte) {
	c.fetchedChunks.Put(chunkID, chunk)
}

// Called when pruning chunks from accepted blocks
//
// Chunks should be pruned AFTER this is called
func (c *ChunkManager) SetMin(min uint64) {
	c.sl.Lock()
	c.min = min
	c.lastChanged = time.Now()
	c.sl.Unlock()
}

// Called when a block is accepted
func (c *ChunkManager) Accept(height uint64, chunks []ids.ID) {
	c.sl.Lock()
	c.max = height
	c.lastChanged = time.Now()
	for _, chunk := range chunks {
		// TODO: Ensure chunks are persisted before this is called
		c.fetchedChunks.Evict(chunk)
	}
	c.sl.Unlock()
}

func (c *ChunkManager) RequestChunk(ctx context.Context, height uint64, chunkID ids.ID) ([]byte, error) {
	if chunk, ok := c.fetchedChunks.Get(chunkID); ok {
		return chunk, nil
	}

	// Check if outstanding request
	c.l.Lock()
	if c.outstanding.Contains(chunkID) {
		c.l.Unlock()
		return nil, nil
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
		c.l.Unlock()
		c.vm.snowCtx.Log.Warn("chunk missing", zap.Stringer("chunkID", chunkID))
		return nil, nil
	}
	recipient := possibleRecipients[rand.Intn(len(possibleRecipients))]
	c.outstanding.Add(chunkID)
	requestID := c.requestID
	c.requestID++
	c.requests[requestID] = chunkID
	c.l.Unlock()

	if err := c.appSender.SendAppRequest(
		ctx,
		set.Set[ids.NodeID]{recipient: struct{}{}},
		requestID,
		chunkID[:],
	); err != nil {
		return nil, err
	}
	return nil, nil
}

func (c *ChunkManager) AppRequest(
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
	if chunk, ok := c.fetchedChunks.Get(chunkID); ok {
		return c.appSender.SendAppResponse(ctx, nodeID, requestID, chunk)
	}
	// TODO: check storage
	// TODO: check pinned chunks in verified blocks (persist here instead of on
	// blocks)
	return c.appSender.SendAppResponse(ctx, nodeID, requestID, nil)
}

func (c *ChunkManager) HandleResponse(nodeID ids.NodeID, requestID uint32, msg []byte) error {
	c.l.Lock()
	request, ok := c.requests[requestID]
	if !ok {
		c.l.Unlock()
		return nil
	}
	delete(c.requests, requestID)
	c.outstanding.Remove(request)
	c.l.Unlock()
	if len(msg) == 0 {
		c.vm.snowCtx.Log.Warn("unexpected chunk not found", zap.Stringer("nodeID", nodeID))
		// TODO: consider retry
		return nil
	}
	chunkID := utils.ToID(msg)
	if chunkID != request {
		c.vm.snowCtx.Log.Warn("received incorrect chunk", zap.Stringer("nodeID", nodeID))
		return nil
	}
	c.fetchedChunks.Put(chunkID, msg)
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
	c.outstanding.Remove(request)
	c.l.Unlock()
	// TODO: consider retry
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

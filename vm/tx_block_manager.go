package vm

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/heap"
	"github.com/ava-labs/hypersdk/utils"
	"go.uber.org/zap"
)

// TODO: make max retries and failure sleep configurable
const (
	maxChunkRetries = 20
	retrySleep      = 50 * time.Millisecond
	gossipFrequency = 100 * time.Millisecond
)

type NodeChunks struct {
	Min uint64
	Max uint64
}

func (n *NodeChunks) Marshal() ([]byte, error) {
	p := codec.NewWriter(consts.Uint64Len*2, consts.NetworkSizeLimit)
	p.PackUint64(n.Min)
	p.PackUint64(n.Max)
	return p.Bytes(), p.Err()
}

func UnmarshalNodeChunks(b []byte) (*NodeChunks, error) {
	var n NodeChunks
	p := codec.NewReader(b, consts.NetworkSizeLimit)
	n.Min = p.UnpackUint64(false) // could be genesis
	n.Max = p.UnpackUint64(false) // could be genesis
	return &n, p.Err()
}

type bucket struct {
	h     uint64          // Height
	items set.Set[ids.ID] // Array of AvalancheGo ids
}

type blkItem struct {
	blk *chain.StatelessTxBlock

	recorded bool
	added    time.Time

	// TODO: consider adding to txBlock directly
	verified atomic.Bool
}

type TxBlockMap struct {
	vm *VM
	l  sync.RWMutex

	bh      *heap.Heap[*bucket, uint64]
	items   map[ids.ID]*blkItem
	heights map[uint64]*bucket // Uses timestamp as keys to map to buckets of ids.

	outstanding set.Set[ids.ID]
}

func NewTxBlockMap(vm *VM) *TxBlockMap {
	// If lower height is accepted and chunk in rejected block that shows later,
	// must not remove yet.
	return &TxBlockMap{
		vm: vm,

		items:       map[ids.ID]*blkItem{},
		heights:     make(map[uint64]*bucket),
		bh:          heap.New[*bucket, uint64](120, true),
		outstanding: set.NewSet[ids.ID](64),
	}
}

// TODO: don't store in block map unless can fetch ancestry back to known block
func (c *TxBlockMap) Add(txBlock *chain.StatelessTxBlock, verified bool) bool {
	c.l.Lock()
	defer c.l.Unlock()

	c.outstanding.Remove(txBlock.ID())

	// Ensure txBlock is not already registered
	b, ok := c.heights[txBlock.Hght]
	if ok && b.items.Contains(txBlock.ID()) {
		return false
	}

	// Add to items
	item := &blkItem{blk: txBlock, added: time.Now()}
	c.items[txBlock.ID()] = item
	if ok {
		// Check if bucket with height already exists
		b.items.Add(txBlock.ID())
	} else {
		// Create new bucket
		b = &bucket{
			h:     txBlock.Hght,
			items: set.Set[ids.ID]{txBlock.ID(): struct{}{}},
		}
		c.heights[txBlock.Hght] = b
		c.bh.Push(&heap.Entry[*bucket, uint64]{
			ID:    txBlock.ID(),
			Val:   txBlock.Hght,
			Item:  b,
			Index: c.bh.Len(),
		})
	}
	if verified {
		// TODO: handle the case where we want to verify others here (seems like it
		// shouldn't happen after issue but may be an invariant to support)?
		item.verified.Store(true)
	}
	return true
}

func (c *TxBlockMap) Verified(blkID ids.ID, success bool) []ids.ID {
	c.l.Lock()
	defer c.l.Unlock()

	// Scan all items at height + 1 that rely on
	blk := c.items[blkID]
	if success {
		blk.verified.Store(true)
	} else {
		return nil
	}

	bucket, ok := c.heights[blk.blk.Hght+1]
	if !ok {
		return nil
	}
	toVerify := []ids.ID{}
	for cblkID := range bucket.items {
		cblk := c.items[cblkID]
		if cblk.blk.Prnt != blkID {
			continue
		}
		toVerify = append(toVerify, cblkID)
	}
	return toVerify
}

func (c *TxBlockMap) Get(blkID ids.ID) *blkItem {
	c.l.RLock()
	defer c.l.RUnlock()

	blk, ok := c.items[blkID]
	if !ok {
		return nil
	}
	return blk
}

func (c *TxBlockMap) SetMin(h uint64) []ids.ID {
	c.l.Lock()
	defer c.l.Unlock()

	evicted := []ids.ID{}
	for {
		b := c.bh.First()
		if b == nil || b.Val > h {
			break
		}
		c.bh.Pop()
		for chunkID := range b.Item.items {
			item := c.items[chunkID]
			if !item.verified.Load() {
				c.vm.metrics.txBlocksDropped.Inc()
			}
			delete(c.items, chunkID)
			evicted = append(evicted, chunkID)
		}
		// Delete from times map
		delete(c.heights, b.Val)
	}
	return evicted
}

// TODO: allow multiple concurrent fetches
func (c *TxBlockMap) Fetch(blkID ids.ID) bool {
	c.l.Lock()
	defer c.l.Unlock()

	_, ok := c.items[blkID]
	if ok {
		return false
	}
	if c.outstanding.Contains(blkID) {
		return false
	}
	c.outstanding.Add(blkID)
	return true
}

func (c *TxBlockMap) Tracking(blkID ids.ID) bool {
	c.l.RLock()
	defer c.l.RUnlock()

	_, ok := c.items[blkID]
	if ok {
		return true
	}
	return c.outstanding.Contains(blkID)
}

func (c *TxBlockMap) AbandonFetch(blkID ids.ID) {
	c.l.Lock()
	defer c.l.Unlock()

	c.outstanding.Remove(blkID)
}

type TxBlockManager struct {
	vm        *VM
	appSender common.AppSender

	requestLock sync.Mutex
	requestID   uint32
	requests    map[uint32]chan []byte

	txBlocks *TxBlockMap
	min      uint64
	max      uint64

	nodeChunkLock sync.RWMutex
	nodeChunks    map[ids.NodeID]*NodeChunks
	nodeSet       set.Set[ids.NodeID]

	update chan []byte
	verify chan ids.ID
	wg     sync.WaitGroup
}

func NewTxBlockManager(vm *VM) *TxBlockManager {
	return &TxBlockManager{
		vm:         vm,
		requests:   map[uint32]chan []byte{},
		txBlocks:   NewTxBlockMap(vm),
		nodeChunks: map[ids.NodeID]*NodeChunks{},
		nodeSet:    set.NewSet[ids.NodeID](64),
		update:     make(chan []byte, 256),
		verify:     make(chan ids.ID, 256),
	}
}

func (c *TxBlockManager) Run(appSender common.AppSender) {
	c.appSender = appSender

	// TxBlock gossiper
	c.wg.Add(1)
	go func() {
		c.vm.Logger().Info("starting chunk gossiper")
		defer c.wg.Done()

		timer := time.NewTicker(gossipFrequency)
		defer timer.Stop()

		for {
			var msg []byte
			select {
			case b := <-c.update:
				msg = b
			case <-timer.C:
			case <-c.vm.stop:
				c.vm.Logger().Info("stopping chunk gossiper")
				return
			}
			if len(msg) == 0 {
				nc := &NodeChunks{
					Min: c.min,
					Max: c.max,
				}
				b, err := nc.Marshal()
				if err != nil {
					c.vm.snowCtx.Log.Warn("unable to marshal chunk gossip", zap.Error(err))
					continue
				}
				msg = append([]byte{0}, b...)
			} else {
				msg = append([]byte{1}, msg...)
				c.vm.metrics.txBlockBytesSent.Add(float64(len(msg) * c.nodeSet.Len()))
			}
			// TODO: need to lock nodeSet?
			// TODO: better to send message one at a time (probably better in a group
			// to avoid gRPC overhead)?
			if err := c.appSender.SendAppGossipSpecific(context.TODO(), c.nodeSet, msg); err != nil {
				c.vm.snowCtx.Log.Warn("unable to send gossip", zap.Error(err))
				continue
			}
		}
	}()

	// TxBlock verifier
	c.wg.Add(1)
	go func() {
		c.vm.Logger().Info("starting chunk verifier")
		defer c.wg.Done()

		for {
			select {
			case blockID := <-c.verify:
				c.VerifyAll(blockID)
			case <-c.vm.stop:
				c.vm.Logger().Info("exiting chunk verifier")
				return
			}
		}
	}()
}

// Called when building a chunk
func (c *TxBlockManager) IssueTxBlock(ctx context.Context, txBlock *chain.StatelessTxBlock) {
	c.txBlocks.Add(txBlock, true)
	msg := txBlock.Bytes()
	c.update <- msg
	if txBlock.Hght > c.max {
		c.max = txBlock.Hght
	}
	c.update <- nil
}

// Called when pruning chunks from accepted blocks
//
// Chunks should be pruned AFTER this is called
// TODO: Set when pruning blobs
// TODO: Set when state syncing
func (c *TxBlockManager) SetMin(min uint64) {
	c.min = min
	c.update <- nil
}

// Called when a block is accepted
//
// Ensure chunks are persisted before calling this method
func (c *TxBlockManager) Accept(height uint64) {
	evicted := c.txBlocks.SetMin(height)
	c.update <- nil
	c.vm.snowCtx.Log.Info("evicted chunks from memory", zap.Int("n", len(evicted)))
}

// TODO: pre-store chunks on disk if bootstrapping
// TODO: change context?
// Each time we attempt to verify a block, we will kickoff fetch if we don't
// already have, eventually verifying
func (c *TxBlockManager) RequireTxBlocks(ctx context.Context, minTxBlkHeight uint64, blkIDs []ids.ID) int {
	missing := 0
	for i, rblkID := range blkIDs {
		blkID := rblkID
		if !c.txBlocks.Tracking(blkID) {
			missing++
		}
		next := minTxBlkHeight + uint64(i)
		if next > 0 {
			go c.RequestTxBlock(ctx, next, ids.EmptyNodeID, blkID, i == 0)
		}
	}
	return missing
}

func (c *TxBlockManager) RetryVerify(ctx context.Context, blkIDs []ids.ID) {
	for _, blkID := range blkIDs {
		if item := c.txBlocks.Get(blkID); item != nil {
			// Stop gap to see if we are just dropping verification accidentally
			//
			// TODO: should not be necessry to re-prompt verify?
			if !item.verified.Load() {
				c.verify <- blkID
			}
		}
	}
}

// RequestChunk may spawn a goroutine
func (c *TxBlockManager) RequestTxBlock(ctx context.Context, height uint64, hint ids.NodeID, blkID ids.ID, recursive bool) {
	shouldFetch := c.txBlocks.Fetch(blkID)
	if !shouldFetch {
		return
	}

	// Attempt to fetch
	for i := 0; i < maxChunkRetries; i++ {
		if err := ctx.Err(); err != nil {
			c.txBlocks.AbandonFetch(blkID)
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
				if height >= chunk.Min && height <= chunk.Max {
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
				c.vm.snowCtx.Log.Warn("no possible recipients", zap.Stringer("blkID", blkID), zap.Stringer("hint", hint), zap.Uint64("height", height))
			}
			peer = randomRecipient
		}

		// Handle received message
		msg, err := c.requestTxBlockNodeID(ctx, peer, height, blkID)
		if err != nil {
			time.Sleep(retrySleep)
			continue
		}
		c.vm.metrics.txBlockBytesReceived.Add(float64(len(msg)))
		if err := c.handleBlock(ctx, msg, &height, hint, recursive); err != nil {
			time.Sleep(retrySleep)
			continue
		}
		return
	}
	c.txBlocks.AbandonFetch(blkID)
}

func (c *TxBlockManager) requestTxBlockNodeID(ctx context.Context, recipient ids.NodeID, height uint64, blkID ids.ID) ([]byte, error) {
	// Send request
	rch := make(chan []byte)
	c.requestLock.Lock()
	requestID := c.requestID
	c.requestID++
	c.requests[requestID] = rch
	c.requestLock.Unlock()
	req := codec.NewWriter(consts.Uint64Len+consts.IDLen, consts.Uint64Len+consts.IDLen)
	req.PackUint64(height)
	req.PackID(blkID)
	if err := req.Err(); err != nil {
		c.vm.snowCtx.Log.Warn("chunk fetch request failed", zap.Stringer("blkID", blkID), zap.Error(err))
		return nil, err
	}
	if err := c.appSender.SendAppRequest(
		ctx,
		set.Set[ids.NodeID]{recipient: struct{}{}},
		requestID,
		req.Bytes(),
	); err != nil {
		c.vm.snowCtx.Log.Warn("chunk fetch request failed", zap.Stringer("blkID", blkID), zap.Error(err))
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
		// Happens if recipient does not have the chunk we want
		c.vm.snowCtx.Log.Warn("chunk fetch returned empty", zap.Stringer("blkID", blkID))
		return nil, errors.New("not found")
	}
	fblkID := utils.ToID(msg)
	if blkID != fblkID {
		// TODO: penalize sender
		c.vm.snowCtx.Log.Warn("received incorrect blockID", zap.Stringer("nodeID", recipient))
		return nil, errors.New("invalid tx block")
	}
	return msg, nil
}

func (c *TxBlockManager) HandleRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	request []byte,
) error {
	p := codec.NewReader(request, consts.Uint64Len+consts.IDLen)
	txBlkHght := p.UnpackUint64(true)
	var txBlkID ids.ID
	p.UnpackID(true, &txBlkID)
	if err := p.Err(); err != nil {
		c.vm.snowCtx.Log.Warn("unable to parse chunk request", zap.Error(err))
		return nil
	}

	// Check processing
	if txBlk := c.txBlocks.Get(txBlkID); txBlk != nil {
		msg := txBlk.blk.Bytes()
		c.vm.metrics.txBlockBytesSent.Add(float64(len(msg)))
		return c.appSender.SendAppResponse(ctx, nodeID, requestID, msg)
	}

	// Check accepted
	txBlk, err := c.vm.GetTxBlock(txBlkHght)
	if err != nil {
		c.vm.snowCtx.Log.Warn("unable to find txBlock", zap.Stringer("txBlkID", txBlkID), zap.Error(err))
		return c.appSender.SendAppResponse(ctx, nodeID, requestID, []byte{})
	}
	msg := txBlk.Bytes()
	c.vm.metrics.txBlockBytesSent.Add(float64(len(msg)))
	return c.appSender.SendAppResponse(ctx, nodeID, requestID, msg)
}

func (c *TxBlockManager) HandleResponse(nodeID ids.NodeID, requestID uint32, msg []byte) error {
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

func (c *TxBlockManager) HandleRequestFailed(requestID uint32) error {
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

func (c *TxBlockManager) HandleAppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	if len(msg) == 0 {
		return nil
	}
	switch msg[0] {
	case 0:
		nc, err := UnmarshalNodeChunks(msg[1:])
		if err != nil {
			c.vm.Logger().Error("unable to parse gossip", zap.Error(err))
			return nil
		}
		c.nodeChunkLock.Lock()
		c.nodeChunks[nodeID] = nc
		c.nodeChunkLock.Unlock()
	case 1:
		b := msg[1:]
		blkID := utils.ToID(b)
		c.vm.metrics.txBlockBytesReceived.Add(float64(len(b)))

		// Option 0: already have txBlock, drop
		if !c.txBlocks.Fetch(blkID) {
			return nil
		}

		// Don't yet have txBlock in cache, figure out what to do
		if err := c.handleBlock(context.TODO(), b, nil, nodeID, true); err != nil {
			c.txBlocks.AbandonFetch(blkID)
			c.vm.Logger().Error("unable to handle txBlock", zap.Error(err))
			return nil
		}
		c.vm.Logger().Info("received tx block gossip", zap.Stringer("blkID", blkID))
	default:
		c.vm.Logger().Error("unexpected message type", zap.Uint8("type", msg[0]))
		return nil
	}
	return nil
}

func (c *TxBlockManager) handleBlock(ctx context.Context, msg []byte, expected *uint64, hint ids.NodeID, recursive bool) error {
	rtxBlk, err := chain.UnmarshalTxBlock(msg, c.vm)
	if err != nil {
		return err
	}
	if rtxBlk.Hght <= c.vm.LastAcceptedBlock().MaxTxHght() && c.vm.LastAcceptedBlock().Hght > 0 {
		// TODO: could explain why we don't have block?
		return nil
	}
	if expected != nil && rtxBlk.Hght != *expected {
		// We stop fetching here because invalid ancestry
		// TODO: mark so we don't go retry
		return errors.New("unexpected height")
	}
	txBlk, err := chain.ParseTxBlock(ctx, rtxBlk, msg, c.vm)
	if err != nil {
		return err
	}
	if !c.txBlocks.Add(txBlk, false) {
		return nil
	}
	if recursive && txBlk.Hght > 1 && (txBlk.Hght-1 > c.vm.LastAcceptedBlock().MaxTxHght() || c.vm.LastAcceptedBlock().Hght == 0) {
		// TODO: don't do recursively to avoid stack blowup
		c.RequestTxBlock(ctx, txBlk.Hght-1, hint, txBlk.Prnt, recursive)
	}
	// TODO: only send verify if RequestTxBlock resolves
	c.verify <- txBlk.ID()
	return nil
}

func (c *TxBlockManager) VerifyAll(blkID ids.ID) {
	next := []ids.ID{blkID}
	for len(next) > 0 {
		nextRound := []ids.ID{}
		for _, blkID := range next {
			err := c.Verify(blkID)
			if err != nil {
				c.vm.Logger().Warn("manager block verification failed", zap.Error(err))
				c.vm.metrics.txBlocksVerifiedFailedManager.Inc()
			} else {
				c.vm.Logger().Info("manager block verification success", zap.Stringer("blkID", blkID))
			}
			nextRound = append(nextRound, c.txBlocks.Verified(blkID, err == nil)...)
		}
		next = nextRound
	}
}

func (c *TxBlockManager) Verify(blkID ids.ID) error {
	blk := c.txBlocks.Get(blkID)
	if blk == nil {
		return fmt.Errorf("tx block is missing: %v", blkID)
	}
	if blk.verified.Load() {
		return errors.New("tx block already verified")
	}
	parent := c.txBlocks.Get(blk.blk.Prnt)
	if parent != nil {
		if !parent.verified.Load() {
			return errors.New("parent tx block not verified")
		}
	} else {
		// Ensure on-disk if not in-memory
		var prntHght uint64
		if blk.blk.Hght > 0 {
			// TODO: consider making a helper
			prntHght = blk.blk.Hght - 1
		}
		_, err := c.vm.GetTxBlock(prntHght)
		if err != nil {
			return err
		}
	}
	if !blk.recorded {
		blk.recorded = true
		c.vm.metrics.addVerifyDiff.Observe(float64(time.Since(blk.added)))
	}
	if err := blk.blk.Verify(context.Background()); err != nil {
		c.vm.Logger().Error("blk.blk.Verify failed", zap.Error(err))
		c.vm.metrics.txBlocksVerifiedFailed.Inc()
		return err
	}
	if blk.blk.Hght > c.max {
		// Only update if verified up to that height
		c.max = blk.blk.Hght
		c.update <- nil
	}
	return nil
}

// Send info to new peer on handshake
func (c *TxBlockManager) HandleConnect(ctx context.Context, nodeID ids.NodeID) error {
	if c.vm.NodeID() == nodeID {
		return nil
	}

	nc := &NodeChunks{
		Min: c.min,
		Max: c.max,
	}
	b, err := nc.Marshal()
	if err != nil {
		c.vm.snowCtx.Log.Warn("unable to marshal chunk gossip specific ", zap.Error(err))
		return nil
	}
	if err := c.appSender.SendAppGossipSpecific(context.TODO(), set.Set[ids.NodeID]{nodeID: struct{}{}}, append([]byte{0}, b...)); err != nil {
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
func (c *TxBlockManager) HandleDisconnect(ctx context.Context, nodeID ids.NodeID) error {
	if c.vm.NodeID() == nodeID {
		return nil
	}

	c.nodeChunkLock.Lock()
	delete(c.nodeChunks, nodeID)
	c.nodeSet.Remove(nodeID)
	c.nodeChunkLock.Unlock()
	return nil
}

func (c *TxBlockManager) Done() {
	c.wg.Wait()
}

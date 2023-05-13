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
	"github.com/ava-labs/hypersdk/chain"
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
	Min uint64
	Max uint64
}

func (n *NodeChunks) Marshal() ([]byte, error) {
	p := codec.NewWriter(consts.NetworkSizeLimit)
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
	h     uint64          // Timestamp
	items set.Set[ids.ID] // Array of AvalancheGo ids
}

type txBlockInfo struct {
	count   int
	txBlock *chain.StatelessTxBlock
}

type TxBlockMap struct {
	l sync.RWMutex

	bh      *heap.Heap[*bucket, uint64]
	counts  map[ids.ID]*txBlockInfo
	heights map[uint64]*bucket // Uses timestamp as keys to map to buckets of ids.
}

func NewTxBlockMap() *TxBlockMap {
	// If lower height is accepted and chunk in rejected block that shows later,
	// must not remove yet.
	return &TxBlockMap{
		counts:  map[ids.ID]*txBlockInfo{},
		heights: make(map[uint64]*bucket),
		bh:      heap.New[*bucket, uint64](120, true),
	}
}

// TODO: don't store in block map unless can fetch ancestry back to known block
func (c *TxBlockMap) Add(txBlock *chain.StatelessTxBlock) {
	c.l.Lock()
	defer c.l.Unlock()

	// Ensure txBlock is not already registered at height
	b, ok := c.heights[txBlock.Hght]
	if ok && b.items.Contains(txBlock.ID()) {
		return
	}

	// Increase chunk count
	info, cok := c.counts[txBlock.ID()]
	if !cok {
		info = &txBlockInfo{txBlock: txBlock}
	}
	info.count++
	c.counts[txBlock.ID()] = info

	// Check if bucket with height already exists
	if ok {
		b.items.Add(txBlock.ID())
		return
	}

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

func (c *TxBlockMap) Get(blkID ids.ID) *chain.StatelessTxBlock {
	c.l.RLock()
	defer c.l.RUnlock()

	info, ok := c.counts[blkID]
	if !ok {
		return nil
	}
	return info.txBlock
}

func (c *TxBlockMap) SetMin(h uint64) []ids.ID {
	c.l.Lock()
	defer c.l.Unlock()

	evicted := []ids.ID{}
	for {
		b := c.bh.First()
		if b == nil || b.Val >= h {
			break
		}
		c.bh.Pop()
		for chunkID := range b.Item.items {
			info := c.counts[chunkID]
			info.count--
			if info.count == 0 {
				delete(c.counts, chunkID)
				evicted = append(evicted, chunkID)
			} else {
				c.counts[chunkID] = info
			}
		}
		// Delete from times map
		delete(c.heights, b.Val)
	}
	return evicted
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

	outstandingLock sync.Mutex
	outstanding     map[ids.ID][]chan *chunkResult

	update chan []byte
	done   chan struct{}
}

func NewTxBlockManager(vm *VM) *TxBlockManager {
	return &TxBlockManager{
		vm:          vm,
		requests:    map[uint32]chan []byte{},
		txBlocks:    NewTxBlockMap(),
		nodeChunks:  map[ids.NodeID]*NodeChunks{},
		nodeSet:     set.NewSet[ids.NodeID](64),
		outstanding: map[ids.ID][]chan *chunkResult{},
		update:      make(chan []byte),
		done:        make(chan struct{}),
	}
}

func (c *TxBlockManager) Run(appSender common.AppSender) {
	c.appSender = appSender

	c.vm.Logger().Info("starting chunk manager")
	defer close(c.done)

	timer := time.NewTicker(gossipFrequency)
	defer timer.Stop()

	for {
		var msg []byte
		select {
		case b := <-c.update:
			msg = b
		case <-timer.C:
		case <-c.vm.stop:
			c.vm.Logger().Info("stopping chunk manager")
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
			msg = b
		}
		if err := c.appSender.SendAppGossipSpecific(context.TODO(), c.nodeSet, msg); err != nil {
			c.vm.snowCtx.Log.Warn("unable to send gossip", zap.Error(err))
			continue
		}
	}
}

// Called when building a chunk
func (c *TxBlockManager) IssueTxBlock(ctx context.Context, txBlock *chain.StatelessTxBlock) {
	c.txBlocks.Add(txBlock)
	c.update <- txBlock.Bytes()
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
	evicted := c.txBlocks.SetMin(height + 1)
	c.update <- nil
	c.vm.snowCtx.Log.Info("evicted chunks from memory", zap.Int("n", len(evicted)))
}

func (c *TxBlockManager) RequestChunks(ctx context.Context, txBlkHeight uint64, txBlkIDs []ids.ID, ch chan []byte) error {
	// TODO: pre-store chunks on disk if bootstrapping
	g, gctx := errgroup.WithContext(ctx)
	for ri, rtxBlkID := range txBlkIDs {
		i := uint64(ri)
		txBlkID := rtxBlkID
		g.Go(func() error {
			crch := make(chan *txBlockResult, 1)
			c.RequestChunk(gctx, txBlkHeight+i, ids.EmptyNodeID, txBlkID, crch)
			select {
			case r := <-crch:
				if r.err != nil {
					return r.err
				}
				// TODO: need to actually return?
				ch <- r.txBlock.Bytes()
				return nil
			case <-gctx.Done():
				return gctx.Err()
			}
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}

	// Trigger that we have processed new chunks
	c.update <- nil
	return nil
}

type txBlockResult struct {
	txBlock *chain.StatelessTxBlock
	err     error
}

func (c *TxBlockManager) sendToOutstandingListeners(txBlockID ids.ID, txBlock *chain.StatelessTxBlock, err error) {
	c.outstandingLock.Lock()
	listeners, ok := c.outstanding[txBlockID]
	delete(c.outstanding, txBlockID)
	c.outstandingLock.Unlock()
	if !ok {
		return
	}
	result := &txBlockResult{txBlock, err}
	for _, listener := range listeners {
		if listener == nil {
			continue
		}
		listener <- result
	}
}

// RequestChunk may spawn a goroutine
func (c *TxBlockManager) RequestChunk(ctx context.Context, height uint64, hint ids.NodeID, chunkID ids.ID, ch chan *txBlockResult) {
	// Register request to be notified
	c.outstandingLock.Lock()
	outstanding, ok := c.outstanding[chunkID]
	if ok {
		c.outstanding[chunkID] = append(outstanding, ch)
	} else {
		c.outstanding[chunkID] = []chan *txBlockResult{ch}
	}
	c.outstandingLock.Unlock()
	if ok {
		// Wait for requests to eventually return
		return
	}

	// Check if previously fetched
	if txBlock := c.txBlocks.Get(chunkID); txBlock != nil {
		c.sendToOutstandingListeners(chunkID, txBlock, nil)
		return
	}

	// Check if optimistically cached
	// TODO: store chunks we've received but not connected yet here to make sure
	// we don't fetch
	// if chunk, ok := c.optimisticChunks.Get(chunkID); ok {
	// 	c.chunkLock.Lock()
	// 	if height != nil {
	// 		c.fetchedChunks[chunkID] = chunk
	// 		c.chunks.Add(*height, chunkID)
	// 	}
	// 	c.chunkLock.Unlock()
	// 	c.sendToOutstandingListeners(chunkID, chunk, nil)
	// 	return
	// }

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
				c.vm.snowCtx.Log.Warn("no possible recipients", zap.Stringer("chunkID", chunkID), zap.Stringer("hint", hint), zap.Uint64("height", height))
			}
			peer = randomRecipient
		}

		// Handle received message
		msg, err := c.requestChunkNodeID(ctx, peer, chunkID)
		if err != nil {
			time.Sleep(retrySleep)
			continue
		}
		rtxBlk, err := chain.UnmarshalTxBlock(msg, c.vm)
		if err != nil {
			c.vm.snowCtx.Log.Warn("invalid tx block", zap.Error(err))
			time.Sleep(retrySleep)
			continue
		}
		txBlk, err := chain.ParseTxBlock(ctx, rtxBlk, msg, c.vm)
		if err != nil {
			c.vm.snowCtx.Log.Warn("unable to init tx block", zap.Error(err))
			time.Sleep(retrySleep)
			continue
		}
		c.txBlocks.Add(txBlk)
		c.sendToOutstandingListeners(chunkID, txBlk, nil)
		return
	}
	c.sendToOutstandingListeners(chunkID, nil, errors.New("exhausted retries"))
}

func (c *TxBlockManager) requestChunkNodeID(ctx context.Context, recipient ids.NodeID, chunkID ids.ID) ([]byte, error) {

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
		// Happens if recipient does not have the chunk we want
		c.vm.snowCtx.Log.Warn("chunk fetch returned empty", zap.Stringer("chunkID", chunkID))
		return nil, errors.New("not found")
	}
	fchunkID := utils.ToID(msg)
	if chunkID != fchunkID {
		// TODO: penalize sender
		c.vm.snowCtx.Log.Warn("received incorrect chunk", zap.Stringer("nodeID", recipient))
		return nil, errors.New("invalid chunk")
	}
	return msg, nil
}

func (c *TxBlockManager) HandleRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	request []byte,
) error {
	txBlkID, err := ids.ToID(request)
	if err != nil {
		c.vm.snowCtx.Log.Warn("unable to parse chunk request", zap.Error(err))
		return nil
	}

	// Check processing
	if txBlk := c.txBlocks.Get(txBlkID); txBlk != nil {
		return c.appSender.SendAppResponse(ctx, nodeID, requestID, txBlk.Bytes())
	}

	// Check accepted
	txBlk, err := c.vm.GetTxBlock(txBlkID)
	if err != nil {
		c.vm.snowCtx.Log.Warn("unable to find txBlock", zap.Stringer("txBlkID", txBlkID), zap.Error(err))
		return c.appSender.SendAppResponse(ctx, nodeID, requestID, []byte{})
	}
	return c.appSender.SendAppResponse(ctx, nodeID, requestID, txBlk)
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

		// Option 0: already have txBlock, drop
		if txBlk := c.txBlocks.Get(blkID); txBlk != nil {
			return nil
		}

		// Don't yet have txBlock in cache, figure out what to do
		txBlock, err := chain.UnmarshalTxBlock(b, c.vm)
		if err != nil {
			c.vm.Logger().Error("unable to parse txBlock", zap.Error(err))
			return nil
		}

		// Ensure tx block could be useful
		//
		// TODO: limit how far ahead we will fetch
		if txBlock.Hght <= c.vm.LastAcceptedBlock().MaxTxHght() {
			c.vm.Logger().Debug("dropp useless tx block", zap.Uint64("hght", txBlock.Hght))
			return nil
		}

		// TODO: tx blocks could build off each other arbitrarily, need to put
		// state in each one

		// Option 1: parent txBlock is missing, must fetch
		parent, ok := c.fetchedChunks[txBlock.Prnt]
		if !ok {
			// TODO: trigger verify once returned
			c.RequestChunk(ctx, &(txBlock.Hght - 1), nodeID, txBlock.Prnt, nil)
			return nil
		}

		// TODO: if keep chunk, increase max value

		// Option 2: parent txBlock is final, must create new child state

		// Option 3: parent txBlock exists and is not final, can verify immediately

		// Optimistically fetch chunks
		// TODO: only fetch if from a soon to be producer (i.e. will need to verify
		// a future block)
		// TODO: handle case where already wrote to disk and we are getting old
		// chunks
		for chunkID := range unprocessed {
			if _, ok := c.clearedChunks.Get(chunkID); ok {
				continue
			}
			if _, ok := c.tryOptimisticChunks.Get(chunkID); ok {
				continue
			}
			c.tryOptimisticChunks.Put(chunkID, nil)
			// TODO: limit max concurrency here
			go c.RequestChunk(context.Background(), nil, nodeID, chunkID, nil)
		}
	default:
		c.vm.Logger().Error("unexpected message type", zap.Uint8("type", msg[0]))
		return nil
	}
	return nil
}

// Send info to new peer on handshake
func (c *TxBlockManager) HandleConnect(ctx context.Context, nodeID ids.NodeID) error {
	nc := &NodeChunks{
		Min: c.min,
		Max: c.max,
	}
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
func (c *TxBlockManager) HandleDisconnect(ctx context.Context, nodeID ids.NodeID) error {
	c.nodeChunkLock.Lock()
	delete(c.nodeChunks, nodeID)
	c.nodeSet.Remove(nodeID)
	c.nodeChunkLock.Unlock()
	return nil
}

func (c *TxBlockManager) Done() {
	<-c.done
}

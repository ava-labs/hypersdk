package vm

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/buffer"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/anchor"
	"github.com/ava-labs/hypersdk/cache"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/eheap"
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/opool"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/ava-labs/hypersdk/workers"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
	"golang.org/x/sync/errgroup"
)

const (
	chunkMsg            uint8 = 0x0
	chunkSignatureMsg   uint8 = 0x1
	chunkCertificateMsg uint8 = 0x2
	chunkCertifiedMsg   uint8 = 0x3 // chunk + chunk certificate
	txMsg               uint8 = 0x4

	chunkReq uint8 = 0x0
	// TODO: add support for filtered chunk requests

	minWeightNumerator = 67
	weightDenominator  = 100

	// While the most efficient size to send over the network is ~MTU, the overhead
	// of handling messages causes performance degradation (high CPU usage) at high-throughput.
	// A byproduct of larger batches, however, is that compression is more efficient and we
	// can better leverage batch signature verification.
	gossipTxTargetSize = 256 * units.KiB
	gossipTxPrealloc   = 2_048
	gossipBatchWait    = 100 * time.Millisecond
)

type simpleChunkWrapper struct {
	chunk ids.ID
	slot  int64
}

func (scw *simpleChunkWrapper) ID() ids.ID {
	return scw.chunk
}

func (scw *simpleChunkWrapper) Expiry() int64 {
	return scw.slot
}

type chunkWrapper struct {
	l sync.Mutex

	sent       time.Time
	chunk      *chain.Chunk
	signatures map[string]*chain.ChunkSignature
}

func (cw *chunkWrapper) ID() ids.ID {
	return cw.chunk.ID()
}

func (cw *chunkWrapper) Expiry() int64 {
	return cw.chunk.Slot
}

type seenWrapper struct {
	chunkID ids.ID
	expiry  int64
	seen    int64
}

func (sw *seenWrapper) ID() ids.ID {
	return sw.chunkID
}

func (sw *seenWrapper) Expiry() int64 {
	return sw.expiry
}

type certWrapper struct {
	cert *chain.ChunkCertificate
	seen int64
}

func (cw *certWrapper) ID() ids.ID {
	return cw.cert.ID()
}

// Expiry here is really the time we first saw,
// which is used to ensure we pop it first when iterating over
// the heap.
//
// TODO: change name of [Expiry] to something more generic
func (cw *certWrapper) Expiry() int64 {
	return cw.seen
}

// Store fifo chunks for building
type CertStore struct {
	l sync.Mutex

	minTime int64
	seen    *eheap.ExpiryHeap[*seenWrapper] // only cleared on expiry
	eh      *eheap.ExpiryHeap[*certWrapper] // cleared when we build, updated when we get a more useful cert
}

func NewCertStore() *CertStore {
	return &CertStore{
		seen: eheap.New[*seenWrapper](64), // TODO: add a config
		eh:   eheap.New[*certWrapper](64), // TODO: add a config
	}
}

// We never want to drop a chunk with more signatures, so this isn't an identical implementation
// to the tx repeat case. Because block building is fast, we don't need to worry about this impacting
// performance too much.
func (c *CertStore) StartStream() {
	c.l.Lock()
}

// Stream removes and returns the highest valued item in m.eh.
//
// Stream assumes the lock is already held.
func (c *CertStore) Stream(ctx context.Context) (*chain.ChunkCertificate, bool) { // O(log N)
	first, ok := c.eh.PopMin()
	if !ok {
		return nil, false
	}
	return first.cert, true
}

func (c *CertStore) FinishStream(certs []*chain.ChunkCertificate) {
	c.l.Unlock()
	for _, cert := range certs {
		c.Update(cert)
	}
}

// Called when we get a valid cert or if a block is rejected with valid certs.
//
// If called more than once for the same ChunkID, the cert will be updated if it has
// more signers.
//
// TODO: update if more weight rather than using signer heuristic?
func (c *CertStore) Update(cert *chain.ChunkCertificate) {
	c.l.Lock()
	defer c.l.Unlock()

	// Only keep certs around that could be included.
	if cert.Slot < c.minTime {
		return
	}

	// Record the first time we saw this certificate, so we can properly
	// sort during building.
	firstSeen := int64(0)
	v, ok := c.seen.Get(cert.ID())
	if !ok {
		firstSeen = time.Now().UnixMilli()
		c.seen.Add(&seenWrapper{
			chunkID: cert.ID(),
			expiry:  cert.Slot,
			seen:    firstSeen,
		})
	} else {
		firstSeen = v.seen
	}

	// Store the certificate
	elem, ok := c.eh.Get(cert.ID())
	if !ok {
		c.eh.Add(&certWrapper{
			cert: cert,
			seen: firstSeen,
		})
		return
	}
	// If the existing certificate has more signers than the
	// new certificate, don't update.
	//
	// TODO: we should use weight here, not just number of signers
	if elem.cert.Signers.Len() > cert.Signers.Len() {
		return
	}
	elem.cert = cert
	c.eh.Update(elem)
}

// Called when a block is accepted with valid certs.
func (c *CertStore) SetMin(ctx context.Context, t int64) []*chain.ChunkCertificate {
	c.l.Lock()
	defer c.l.Unlock()

	c.minTime = t
	removedElems := c.seen.SetMin(t)
	certs := make([]*chain.ChunkCertificate, 0, len(removedElems))
	for _, removed := range removedElems {
		cw, ok := c.eh.Remove(removed.ID())
		if !ok {
			continue
		}
		certs = append(certs, cw.cert)
	}
	return certs
}

func (c *CertStore) Get(cid ids.ID) (*chain.ChunkCertificate, bool) {
	c.l.Lock()
	defer c.l.Unlock()

	elem, ok := c.eh.Get(cid)
	if !ok {
		return nil, false
	}
	return elem.cert, true
}

// TODO: move to standalone package
type ChunkManager struct {
	vm   *VM
	done chan struct{}

	appSender      common.AppSender
	incomingChunks chan *chunkGossipWrapper
	incomingTxs    chan *txGossipWrapper

	epochHeights *cache.FIFO[uint64, uint64]

	built *emap.EMap[*chunkWrapper]
	// TODO: rebuild stored on startup
	stored *eheap.ExpiryHeap[*simpleChunkWrapper] // tracks all chunks we've stored, so we can ensure even unused are deleted

	// TODO: track which chunks we've received per nodeID per slot

	certs *CertStore
	auth  *ChunkAuthorizer

	// Ensures that only one request job is running at a time
	waiterL sync.Mutex
	waiter  chan struct{}

	// Handles concurrent fetching of chunks
	callbacksL sync.Mutex
	requestID  uint32
	callbacks  map[uint32]func([]byte)

	// connected includes all connected nodes, not just those that are validators (we use
	// this for sending txs/requesting chunks from only connected nodes)
	connectedL sync.Mutex
	connected  set.Set[ids.NodeID]

	txL     sync.Mutex
	txQueue buffer.Deque[*txGossip]
	txNodes map[ids.NodeID]*txGossip
}

type txGossip struct {
	nodeID ids.NodeID
	txs    map[ids.ID]*chain.Transaction
	size   int
	expiry time.Time
	sent   bool
}

func NewChunkManager(vm *VM) *ChunkManager {
	epochHeights, err := cache.NewFIFO[uint64, uint64](48)
	if err != nil {
		panic(err)
	}
	return &ChunkManager{
		vm:   vm,
		done: make(chan struct{}),

		incomingChunks: make(chan *chunkGossipWrapper, vm.config.GetChunkStorageBacklog()),
		incomingTxs:    make(chan *txGossipWrapper, vm.config.GetAuthGossipBacklog()),

		epochHeights: epochHeights,

		built:  emap.NewEMap[*chunkWrapper](),
		stored: eheap.New[*simpleChunkWrapper](64),

		certs: NewCertStore(),
		auth:  NewChunkAuthorizer(vm),

		callbacks: make(map[uint32]func([]byte)),

		connected: set.NewSet[ids.NodeID](64), // TODO: make a const

		// TODO: move to separate package
		txQueue: buffer.NewUnboundedDeque[*txGossip](64), // TODO: make a const
		txNodes: make(map[ids.NodeID]*txGossip),
	}
}

func (c *ChunkManager) getEpochInfo(ctx context.Context, t int64) (uint64, uint64, error) {
	r := c.vm.Rules(time.Now().UnixMilli())
	epoch := utils.Epoch(t, r.GetEpochDuration())
	if h, ok := c.epochHeights.Get(epoch); ok {
		return epoch, h, nil
	}
	_, heights, err := c.vm.Engine().GetEpochHeights(ctx, []uint64{epoch})
	if err != nil {
		return 0, 0, err
	}
	if heights[0] == nil {
		return 0, 0, errors.New("epoch is not yet set")
	}
	c.epochHeights.Put(epoch, *heights[0])
	return epoch, *heights[0], nil
}

func (c *ChunkManager) Connected(_ context.Context, nodeID ids.NodeID, _ *version.Application) error {
	c.connectedL.Lock()
	defer c.connectedL.Unlock()

	c.connected.Add(nodeID)
	return nil
}

func (c *ChunkManager) Disconnected(_ context.Context, nodeID ids.NodeID) error {
	c.connectedL.Lock()
	defer c.connectedL.Unlock()

	c.connected.Remove(nodeID)
	return nil
}

func (c *ChunkManager) PushSignature(ctx context.Context, nodeID ids.NodeID, sig *chain.ChunkSignature) {
	msg := make([]byte, 1+sig.Size())
	msg[0] = chunkSignatureMsg
	sigBytes, err := sig.Marshal()
	if err != nil {
		c.vm.Logger().Warn("failed to marshal chunk", zap.Error(err))
		return
	}
	copy(msg[1:], sigBytes)
	c.appSender.SendAppGossip(ctx, common.SendConfig{NodeIDs: set.Of(nodeID)}, msg) // skips validators we aren't connected to
}

func (c *ChunkManager) VerifyAndStoreChunk(ctx context.Context, nodeID ids.NodeID, chunk *chain.Chunk) error {
	// Check if we already received
	if c.stored.Has(chunk.ID()) {
		c.vm.metrics.gossipChunkInvalid.Inc()
		return errors.New("already received chunk")
	}

	// Check if chunk < slot
	if chunk.Slot < c.vm.lastAccepted.StatefulBlock.Timestamp {
		c.vm.metrics.gossipChunkInvalid.Inc()
		return errors.New("dropping expired chunk")
	}

	// Check that producer is the sender
	if chunk.Producer != nodeID {
		c.vm.metrics.gossipChunkInvalid.Inc()
		return errors.New("dropping chunk gossip that isn't from producer")
	}

	// Determine if chunk producer is a validator and that their key is valid
	_, epochHeight, err := c.getEpochInfo(ctx, chunk.Slot)
	if err != nil {
		c.vm.metrics.gossipChunkInvalid.Inc()
		return fmt.Errorf("%w: unable to determine chunk epoch", err)
	}
	isValidator, signerKey, _, err := c.vm.proposerMonitor.IsValidator(ctx, epochHeight, chunk.Producer)
	if err != nil {
		c.vm.metrics.gossipChunkInvalid.Inc()
		return errors.New("unable to determine if producer is validator")
	}
	if !isValidator {
		c.vm.metrics.gossipChunkInvalid.Inc()
		return errors.New("dropping chunk gossip from non-validator")
	}
	if signerKey == nil || !bytes.Equal(bls.PublicKeyToCompressedBytes(chunk.Signer), bls.PublicKeyToCompressedBytes(signerKey)) {
		c.vm.metrics.gossipChunkInvalid.Inc()
		return errors.New("dropping validator signed chunk with wrong key")
	}

	// Verify signature of chunk
	if !chunk.VerifySignature(c.vm.snowCtx.NetworkID, c.vm.snowCtx.ChainID) {
		c.vm.metrics.gossipChunkInvalid.Inc()
		return errors.New("dropping chunk with invalid signature")
	}

	// TODO: only store 1 chunk per slot per validator
	// TODO: warn if chunk is dropped for a conflict during fetching (same producer, slot, different chunkID)

	// Persist chunk to disk (delete if not used in time but need to store to protect
	// against shutdown risk across network -> chunk may no longer be accessible after included
	// in referenced certificate)
	if err := c.vm.StoreChunk(chunk); err != nil {
		return fmt.Errorf("%w: unable to persist chunk to disk", err)
	}
	c.stored.Add(&simpleChunkWrapper{chunk: chunk.ID(), slot: chunk.Slot})
	return nil
}

func (c *ChunkManager) VerifyAndTrackCertificate(ctx context.Context, cert *chain.ChunkCertificate) error {
	// Determine epoch for certificate
	_, epochHeight, err := c.getEpochInfo(ctx, cert.Slot)
	if err != nil {
		c.vm.metrics.gossipCertInvalid.Inc()
		return fmt.Errorf("%w: unable to determine certificate epoch", err)
	}

	// Verify certificate using the epoch validator set
	aggrPubKey, err := c.vm.proposerMonitor.GetAggregatePublicKey(ctx, epochHeight, cert.Signers, minWeightNumerator, weightDenominator)
	if err != nil {
		c.vm.metrics.gossipCertInvalid.Inc()
		return fmt.Errorf("%w: unable to get aggregate public key", err)
	}
	if !cert.VerifySignature(c.vm.snowCtx.NetworkID, c.vm.snowCtx.ChainID, aggrPubKey) {
		c.vm.metrics.gossipCertInvalid.Inc()
		return errors.New("dropping invalid certificate")
	}
	c.vm.metrics.certsReceived.Inc()

	// If we don't have the chunk, we wait to fetch it until the certificate is included in an accepted block.

	// TODO: if this certificate conflicts with a chunk we signed, post the conflict (slashable fault)
	// @todo
	c.IssuePreConf(ctx, cert.ID(), epochHeight, set.Bits{})
	// Store chunk certificate for building
	c.certs.Update(cert)
	return nil
}

func (c *ChunkManager) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	start := time.Now()
	if len(msg) == 0 {
		c.vm.Logger().Warn("dropping empty message", zap.Stringer("nodeID", nodeID))
		return nil
	}
	switch msg[0] {
	case chunkMsg:
		c.vm.metrics.chunksReceived.Inc()
		chunk, err := chain.UnmarshalChunk(msg[1:], c.vm)
		if err != nil {
			c.vm.metrics.gossipChunkInvalid.Inc()
			c.vm.Logger().Warn("unable to unmarshal chunk", zap.Stringer("nodeID", nodeID), zap.String("chunk", hex.EncodeToString(msg[1:])), zap.Error(err))
			return nil
		}

		// Enqueue chunk for verification, if we have a backlog drop it
		c.vm.metrics.gossipChunkBacklog.Inc()
		select {
		case c.incomingChunks <- &chunkGossipWrapper{nodeID: nodeID, chunk: chunk}:
			// TODO: dedup incoming chunks to prevent backlog on same chunk (would block filedb)
		default:
			// TODO: track backlog by nodeID
			c.vm.Logger().Warn("dropping chunk gossip because too big of a backlog", zap.Stringer("nodeID", nodeID))
			c.vm.metrics.gossipChunkBacklog.Dec()
			c.vm.metrics.chunkGossipDropped.Inc()
		}

		c.vm.Logger().Debug(
			"received chunk from gossip",
			zap.Stringer("chunkID", chunk.ID()),
			zap.Stringer("nodeID", nodeID),
			zap.Int("size", len(msg)-1),
			zap.Duration("t", time.Since(start)),
		)
	case chunkSignatureMsg:
		c.vm.metrics.sigsReceived.Inc()
		chunkSignature, err := chain.UnmarshalChunkSignature(msg[1:])
		if err != nil {
			c.vm.metrics.gossipChunkSigInvalid.Inc()
			c.vm.Logger().Warn("dropping chunk gossip from non-validator", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}

		// Check if we broadcast this chunk
		cw, ok := c.built.Get(chunkSignature.Chunk)
		if !ok || cw.chunk.Slot != chunkSignature.Slot {
			c.vm.metrics.gossipChunkSigInvalid.Inc()
			c.vm.Logger().Warn("dropping useless chunk signature", zap.Bool("built", ok), zap.Stringer("nodeID", nodeID), zap.Stringer("chunkID", chunkSignature.Chunk))
			return nil
		}

		// Determine if chunk signer is a validator and that their key is valid
		_, epochHeight, err := c.getEpochInfo(ctx, chunkSignature.Slot)
		if err != nil {
			c.vm.metrics.gossipChunkSigInvalid.Inc()
			c.vm.Logger().Warn("unable to determine chunk epoch", zap.Int64("slot", chunkSignature.Slot))
			return nil
		}
		isValidator, signerKey, _, err := c.vm.proposerMonitor.IsValidator(ctx, epochHeight, nodeID)
		if err != nil {
			c.vm.metrics.gossipChunkSigInvalid.Inc()
			c.vm.Logger().Warn("unable to determine if signer is validator", zap.Stringer("signer", nodeID), zap.Error(err))
			return nil
		}
		if !isValidator {
			c.vm.metrics.gossipChunkSigInvalid.Inc()
			c.vm.Logger().Warn("dropping chunk signature from non-validator", zap.Stringer("nodeID", nodeID))
			return nil
		}
		if signerKey == nil || !bytes.Equal(bls.PublicKeyToCompressedBytes(chunkSignature.Signer), bls.PublicKeyToCompressedBytes(signerKey)) {
			c.vm.metrics.gossipChunkSigInvalid.Inc()
			c.vm.Logger().Warn("dropping validator signed chunk with wrong key", zap.Stringer("nodeID", nodeID))
			return nil
		}

		// Verify signature
		if !chunkSignature.VerifySignature(c.vm.snowCtx.NetworkID, c.vm.snowCtx.ChainID) {
			c.vm.metrics.gossipChunkSigInvalid.Inc()
			c.vm.Logger().Warn("dropping chunk signature with invalid signature", zap.Stringer("nodeID", nodeID))
			return nil
		}

		// Store signature for chunk
		cw.l.Lock()
		cw.signatures[string(bls.PublicKeyToCompressedBytes(chunkSignature.Signer))] = chunkSignature // canonical validator set requires fetching signature by bls public key

		// Count pending weight
		var weight uint64
		vdrList, totalWeight, err := c.vm.proposerMonitor.GetWarpValidatorSet(ctx, epochHeight)
		if err != nil {
			panic(err)
		}
		for _, vdr := range vdrList {
			k := string(bls.PublicKeyToCompressedBytes(vdr.PublicKey))
			if _, ok := cw.signatures[k]; ok {
				weight += vdr.Weight // cannot overflow
			}
		}
		cw.l.Unlock()

		// Check if weight is sufficient
		//
		// Fees are proportional to the weight of the chunk, so we may want to wait until it has more than the minimum.
		//
		// TODO: add a timeout here in case we never get above target
		// @todo verifies weight of the total signers in chunk.
		if err := warp.VerifyWeight(weight, totalWeight, c.vm.config.GetMinimumCertificateBroadcastNumerator(), weightDenominator); err != nil {
			c.vm.Logger().Debug("chunk does not have sufficient weight to create certificate", zap.Stringer("chunkID", chunkSignature.Chunk), zap.Error(err))
			return nil
		}

		// Record time to collect and then reset to ensure we don't log multiple times
		firstCertificate := !cw.sent.IsZero()
		if firstCertificate {
			c.vm.metrics.collectChunkSignatures.Observe(float64(time.Since(cw.sent)))
			cw.sent = time.Time{}
		}

		// Construct certificate
		canonicalValidators, _, err := c.vm.proposerMonitor.GetWarpValidatorSet(ctx, epochHeight)
		if err != nil {
			c.vm.Logger().Warn("cannot get canonical validator set", zap.Error(err))
			return nil
		}
		signers := set.NewBits()
		orderedSignatures := []*bls.Signature{}
		cw.l.Lock()
		for i, vdr := range canonicalValidators {
			sig, ok := cw.signatures[string(bls.PublicKeyToCompressedBytes(vdr.PublicKey))]
			if !ok {
				continue
			}
			signers.Add(i)
			orderedSignatures = append(orderedSignatures, sig.Signature)
		}
		cw.l.Unlock()
		aggSignature, err := bls.AggregateSignatures(orderedSignatures)
		if err != nil {
			c.vm.Logger().Warn("cannot generate aggregate signature", zap.Error(err))
			return nil
		}

		// Construct and update stored certificate
		cert := &chain.ChunkCertificate{
			Chunk: chunkSignature.Chunk,
			Slot:  chunkSignature.Slot,

			Signers:   signers,
			Signature: aggSignature,
		}
		c.vm.Logger().Debug(
			"constructed chunk certificate",
			zap.Uint64("Pheight", epochHeight),
			zap.Stringer("chunkID", chunkSignature.Chunk),
			zap.Uint64("weight", weight),
			zap.Uint64("totalWeight", totalWeight),
			zap.Duration("t", time.Since(start)),
		)

		// We don't send our own certs to the optimistic verifier because we
		// don't verify the signatures in those chunks anyways.
		c.certs.Update(cert)

		// Broadcast certificate to validators (we will broadcast each time the number of signers goes up)
		// @todo only broadcast certificate for the first time ever to validators only after gathering minimum required stake signed.
		// keep broadcasting to all validators everytime after increase in the stake signing in the chunk, as validator rewards are propotional to the signatures they gather.
		c.PushChunkCertificate(ctx, cert) // @todo issue preconf in the push chunk certicificate method for produced chunks.

		// Send chunk and cert to non-validators (this will ensure they don't need to fetch chunks and that
		// will allow them to kickoff signature verification earlier).
		//
		// TODO: limit number of nodes we send to/use a whitelist to avoid
		// a sybil attack on our bandwidth (that being said, they'll try to request
		// these from us anyways).
		if !firstCertificate {
			return nil
		}
		// @todo send chunk and certificates to non validators, for the first time, when minimum required stake signs the chunk.
		//https://hackmd.io/@patrickogrady/rys8mdl5p#Chunk-Certificate
		c.PushCertifiedChunk(ctx, cw.chunk, cert)
	case chunkCertificateMsg:
		cert, err := chain.UnmarshalChunkCertificate(msg[1:])
		if err != nil {
			c.vm.metrics.gossipCertInvalid.Inc()
			c.vm.Logger().Warn("dropping chunk gossip from non-validator", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}

		// Handle cert verification
		if err := c.VerifyAndTrackCertificate(ctx, cert); err != nil { // @todo issue preconf in the verify and track certificate method for received chunks.
			c.vm.Logger().Warn("unable to verify and store certificate", zap.Stringer("nodeID", nodeID), zap.Stringer("chunkID", cert.Chunk), zap.Error(err))
			return nil
		}

		// Attempt to add chunk to the optimistic verifier
		//
		// If the chunk has already been added, we just skip it.
		//
		// This operation should be cached, so it should be fast.
		chunk, err := c.vm.GetChunk(cert.Slot, cert.Chunk)
		if chunk == nil {
			// @todo this expects chunk to be available before cert is verified.
			// should nt the chunk be fetched?
			c.vm.Logger().Debug("skipping optimistic chunk auth because chunk is missing", zap.Stringer("chunkID", cert.Chunk), zap.Error(err))
			return nil
		}
		c.auth.Add(chunk)
	case chunkCertifiedMsg:
		chunk, cert, err := parseCertifiedChunkMsg(msg, c.vm)
		if err != nil {
			c.vm.Logger().Warn("dropping invalid certified chunk", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}

		// Enqueue chunk and cert for verification, if we have a backlog drop it
		c.vm.metrics.gossipChunkBacklog.Inc()
		select {
		case c.incomingChunks <- &chunkGossipWrapper{nodeID: nodeID, chunk: chunk, cert: cert}:
			// TODO: dedup incoming chunks to prevent backlog on same chunk (would block filedb)
		default:
			// TODO: track backlog by nodeID
			c.vm.Logger().Warn("dropping chunk and cert gossip because too big of a backlog", zap.Stringer("nodeID", nodeID))
			c.vm.metrics.gossipChunkBacklog.Dec()
			c.vm.metrics.chunkGossipDropped.Inc()
		}
	case txMsg:
		authCounts, txs, err := chain.UnmarshalTxs(msg[1:], gossipTxPrealloc, c.vm.actionRegistry, c.vm.authRegistry)
		if err != nil {
			c.vm.Logger().Warn("dropping invalid tx gossip from non-validator", zap.Stringer("nodeID", nodeID), zap.Error(err))
			c.vm.metrics.gossipTxMsgInvalid.Inc()
			return nil
		}
		txLen := len(txs)
		c.vm.RecordTxsReceived(txLen)

		// Enqueue txs for verification, if we have a backlog just drop them
		c.vm.metrics.gossipTxBacklog.Add(float64(txLen))
		select {
		case c.incomingTxs <- &txGossipWrapper{nodeID: nodeID, txs: txs, authCounts: authCounts}:
		default:
			c.vm.metrics.gossipTxBacklog.Add(-float64(txLen))
			c.vm.metrics.txGossipDropped.Add(float64(txLen))
			c.vm.Logger().Warn("dropping tx gossip because too big of backlog", zap.Stringer("nodeID", nodeID))
		}

		c.vm.Logger().Debug(
			"received txs from gossip",
			zap.Int("txs", len(txs)),
			zap.Stringer("nodeID", nodeID),
			zap.Duration("t", time.Since(start)),
		)
	default:
		c.vm.Logger().Warn("dropping unknown message type", zap.Stringer("nodeID", nodeID))
	}
	return nil
}

func (c *ChunkManager) AppRequest(
	ctx context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	_ time.Time,
	request []byte,
) error {
	if len(request) == 0 {
		c.vm.Logger().Warn("dropping empty message", zap.Stringer("nodeID", nodeID))
		return nil
	}
	switch request[0] {
	case chunkReq:
		rid := request[1:]
		if len(rid) != consts.Uint64Len+ids.IDLen {
			c.vm.Logger().Warn("dropping invalid request", zap.Stringer("nodeID", nodeID))
			return nil
		}
		slot := int64(binary.BigEndian.Uint64(rid[:consts.Uint64Len]))
		id := ids.ID(rid[consts.Uint64Len:])
		chunk, err := c.vm.GetChunkBytes(slot, id)
		if chunk == nil || err != nil {
			c.vm.Logger().Warn(
				"unable to fetch chunk",
				zap.Stringer("nodeID", nodeID),
				zap.Int64("slot", slot),
				zap.Stringer("chunkID", id),
				zap.Error(err),
			)
			c.appSender.SendAppError(ctx, nodeID, requestID, -1, err.Error()) // TODO: add error so caller knows it is missing
			return nil
		}
		c.appSender.SendAppResponse(ctx, nodeID, requestID, chunk)
	default:
		c.vm.Logger().Warn("dropping unknown message type", zap.Stringer("nodeID", nodeID))
	}
	return nil
}

func (w *ChunkManager) AppRequestFailed(
	_ context.Context,
	nodeID ids.NodeID,
	requestID uint32,
) error {
	w.callbacksL.Lock()
	callback, ok := w.callbacks[requestID]
	delete(w.callbacks, requestID)
	w.callbacksL.Unlock()
	if !ok {
		w.vm.Logger().Warn("dropping unknown response", zap.Stringer("nodeID", nodeID), zap.Uint32("requestID", requestID))
		return nil
	}
	callback(nil)
	return nil
}

func (w *ChunkManager) AppError(
	_ context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	_ int32,
	err string,
) error {
	w.callbacksL.Lock()
	callback, ok := w.callbacks[requestID]
	delete(w.callbacks, requestID)
	w.callbacksL.Unlock()
	if !ok {
		w.vm.Logger().Warn("dropping unknown response", zap.Stringer("nodeID", nodeID), zap.Uint32("requestID", requestID))
		return nil
	}
	w.vm.Logger().Warn("dropping request with error", zap.Stringer("nodeID", nodeID), zap.Uint32("requestID", requestID), zap.String("error", err))
	callback(nil)
	return nil
}

func (w *ChunkManager) AppResponse(
	_ context.Context,
	nodeID ids.NodeID,
	requestID uint32,
	response []byte,
) error {
	w.callbacksL.Lock()
	callback, ok := w.callbacks[requestID]
	delete(w.callbacks, requestID)
	w.callbacksL.Unlock()
	if !ok {
		w.vm.Logger().Warn("dropping unknown response", zap.Stringer("nodeID", nodeID), zap.Uint32("requestID", requestID))
		return nil
	}
	callback(response)
	return nil
}

func (*ChunkManager) CrossChainAppRequest(context.Context, ids.ID, uint32, time.Time, []byte) error {
	return nil
}

func (*ChunkManager) CrossChainAppRequestFailed(context.Context, ids.ID, uint32) error {
	return nil
}

func (*ChunkManager) CrossChainAppResponse(context.Context, ids.ID, uint32, []byte) error {
	return nil
}

func (*ChunkManager) CrossChainAppError(context.Context, ids.ID, uint32, int32, string) error {
	return nil
}

type chunkGossipWrapper struct {
	nodeID ids.NodeID
	chunk  *chain.Chunk
	cert   *chain.ChunkCertificate
}

type txGossipWrapper struct {
	nodeID     ids.NodeID
	txs        []*chain.Transaction
	authCounts map[uint8]int
}

func (c *ChunkManager) Run(appSender common.AppSender) {
	c.appSender = appSender
	defer close(c.done)

	beneficiary := c.vm.Beneficiary()
	skipChunks := false
	if bytes.Equal(beneficiary[:], codec.EmptyAddress[:]) {
		c.vm.Logger().Warn("no beneficiary set, not building chunks")
		skipChunks = true
	}
	c.vm.Logger().Info("starting chunk manager", zap.Any("beneficiary", beneficiary))

	// While we could try to shae the same goroutine for some of these items (as they aren't
	// expected to take much time), it is safter to split apart.
	g := &errgroup.Group{}
	g.Go(func() error {
		// TODO: return a proper error if something unexpected happens rather than panic
		c.auth.Run()
		return nil
	})
	g.Go(func() error {
		t := time.NewTicker(c.vm.config.GetChunkBuildFrequency())
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if !c.vm.isReady() {
					c.vm.Logger().Debug("skipping chunk loop because vm isn't ready")
					continue
				}
				if skipChunks {
					continue
				}
				ctx := context.TODO()
				now := time.Now().UnixMilli() - consts.ClockSkewAllowance
				r := c.vm.Rules(now)
				slot := utils.UnixRDeci(now, r.GetValidityWindow()) // chunk validity window is 9 seconds @todo

				_, epochHeight, err := c.getEpochInfo(ctx, slot)
				if err != nil {
					c.vm.metrics.gossipChunkSigInvalid.Inc()
					c.vm.Logger().Warn("unable to determine chunk epoch", zap.Int64("slot", slot))
					return nil
				}

				canonicalValidators, _, err := c.vm.proposerMonitor.GetWarpValidatorSet(ctx, epochHeight)
				if err != nil {
					c.vm.Logger().Warn("cannot get canonical validator set", zap.Error(err))
				}

				totalAnchors := c.vm.AnchorRegistry().Len()

				partition := c.vm.proposerMonitor.PartitionArray(ctx, epochHeight, epochHeight, totalAnchors, len(canonicalValidators))

				myValidatorIdx := 0
				myPubkey := c.vm.snowCtx.PublicKey
				for i, validator := range canonicalValidators {
					if *validator.PublicKey == *myPubkey {
						myValidatorIdx = i
						break
					}
				}
				partitionedAnchors, assigned := partition[myValidatorIdx]
				if !assigned || len(partitionedAnchors) == 0 {
					continue
				}

				anchors := c.vm.AnchorRegistry().SortedAnchors()
				anchorClis := make([]*anchor.Anchor, 0)
				for _, anchorIdx := range partitionedAnchors {
					anchorClis = append(anchorClis, anchors[anchorIdx])
				}

				for _, anchorCli := range anchorClis {
					go func() {
						// // Attempt to build a chunk
						now := time.Now().UnixMilli() - consts.ClockSkewAllowance
						r := c.vm.Rules(now)
						digest, err := anchorCli.RequestAnchorDigest()
						if err != nil {
							c.vm.Logger().Error("unable to fetch chunk digest from anchor", zap.Error(err))
							return
						}
						wm, err := warp.NewUnsignedMessage(r.NetworkID(), r.ChainID(), digest)
						if err != nil {
							c.vm.Logger().Error("unable to generate warp message", zap.Error(err))
							return
						}
						sigBytes, err := c.vm.Sign(wm)
						if err != nil {
							c.vm.Logger().Error("unable to sign message", zap.Error(err))
							return
						}
						sig, err := bls.SignatureFromBytes(sigBytes)
						if err != nil {
							c.vm.Logger().Error("unable to unmarshal sig", zap.Error(err))
							return
						}

						meta, slot, txs, priorityFeeReceiver, err := anchorCli.RequestAnchorChunk(sig)
						if err != nil {
							c.vm.Logger().Error("unable to fetch chunk from Anchor", zap.Error(err))
							return
						}

						chunk, err := chain.BuildChunkFromAnchor(context.TODO(), c.vm, meta, slot, txs, priorityFeeReceiver)
						if err != nil {
							c.vm.Logger().Error("unable to build chunk", zap.Error(err))
							return
						}
						c.auth.Add(chunk) // this will be a no-op because we are the producer
						c.PushChunk(context.TODO(), chunk)

					}()
				}

			case <-c.vm.stop:
				// If engine taking too long to process message, Shutdown will not
				// be called.
				c.vm.Logger().Info("stopping chunk manager")
				return nil
			}
		}
	})

	g.Go(func() error {
		t := time.NewTicker(c.vm.config.GetChunkBuildFrequency())
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if !c.vm.isReady() {
					c.vm.Logger().Debug("skipping chunk loop because vm isn't ready")
					continue
				}
				if skipChunks {
					continue
				}

				// Attempt to build a chunk
				chunkStart := time.Now()
				chunk, err := chain.BuildChunk(context.TODO(), c.vm)
				switch {
				case errors.Is(err, chain.ErrNoTxs) || errors.Is(err, chain.ErrNotAValidator):
					c.vm.Logger().Debug("unable to build chunk", zap.Error(err))
					continue
				case err != nil:
					c.vm.Logger().Error("unable to build chunk", zap.Error(err))
					continue
				default:
					c.vm.Logger().Debug("chunk built", zap.Int("numTxs", len(chunk.Txs)))
					for _, tx := range chunk.Txs {
						c.vm.Logger().Debug("tx in chunk", zap.String("txID", tx.ID().String()))
					}
				}
				c.auth.Add(chunk) // this will be a no-op because we are the producer
				c.PushChunk(context.TODO(), chunk)
				chunkBytes := chunk.Size()
				c.vm.metrics.chunkBuild.Observe(float64(time.Since(chunkStart)))
				c.vm.metrics.chunkBytesBuilt.Add(float64(chunkBytes))
				c.vm.metrics.mempoolLen.Set(float64(c.vm.Mempool().Len(context.TODO())))
				c.vm.metrics.mempoolSize.Set(float64(c.vm.Mempool().Size(context.TODO())))
			case <-c.vm.stop:
				// If engine taking too long to process message, Shutdown will not
				// be called.
				c.vm.Logger().Info("stopping chunk manager")
				return nil
			}
		}
	})
	g.Go(func() error {
		t := time.NewTicker(c.vm.config.GetBlockBuildFrequency())
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if !c.vm.isReady() {
					c.vm.Logger().Info("skipping block loop because vm isn't ready")
					continue
				}

				// Attempt to build a block
				select {
				case c.vm.EngineChan() <- common.PendingTxs:
				default:
				}
			case <-c.vm.stop:
				// If engine taking too long to process message, Shutdown will not
				// be called.
				c.vm.Logger().Info("stopping chunk manager")
				return nil
			}
		}
	})
	g.Go(func() error {
		t := time.NewTicker(50 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				now := time.Now()
				gossipable := []*txGossip{}
				c.txL.Lock()
				for {
					gossip, ok := c.txQueue.PopLeft()
					if !ok {
						break
					}
					if !gossip.expiry.Before(now) {
						c.txQueue.PushLeft(gossip)
						break
					}
					delete(c.txNodes, gossip.nodeID)
					if gossip.sent {
						continue
					}
					gossipable = append(gossipable, gossip)
				}
				c.txL.Unlock()
				for _, gossip := range gossipable {
					c.sendTxGossip(context.TODO(), gossip)
				}
			case <-c.vm.stop:
				// If engine taking too long to process message, Shutdown will not
				// be called.
				c.vm.Logger().Info("stopping chunk manager")
				return nil
			}
		}
	})
	for i := 0; i < c.vm.config.GetAuthGossipCores(); i++ {
		g.Go(func() error {
			for {
				select {
				case txw := <-c.incomingTxs:
					c.vm.metrics.gossipTxBacklog.Add(-float64(len(txw.txs)))
					ctx := context.TODO()
					w := workers.NewSerial()
					job, err := w.NewJob(len(txw.txs))
					if err != nil {
						c.vm.metrics.txGossipDropped.Add(float64(len(txw.txs)))
						c.vm.Logger().Warn("unable to spawn new worker", zap.Error(err))
						continue
					}
					invalid := false
					batchVerifier := chain.NewAuthBatch(c.vm, job, txw.authCounts)
					for _, tx := range txw.txs {
						epoch, epochHeight, err := c.getEpochInfo(ctx, tx.Base.Timestamp)
						if err != nil {
							c.vm.Logger().Debug("unable to determine tx epoch", zap.Int64("t", tx.Base.Timestamp))
							invalid = true
							break
						}
						partition, err := c.vm.proposerMonitor.AddressPartition(ctx, epoch, epochHeight, tx.Sponsor(), tx.Partition())
						if err != nil {
							c.vm.Logger().Debug("unable to compute address partition", zap.Error(err))
							invalid = true
							break
						}
						if partition != c.vm.snowCtx.NodeID {
							c.vm.Logger().Debug("dropping tx gossip from non-partition", zap.Stringer("nodeID", txw.nodeID))
							invalid = true
							break
						}
						// Verify signature async
						msg, err := tx.Digest()
						if err != nil {
							c.vm.Logger().Warn(
								"unable to compute tx digest",
								zap.Stringer("peerID", txw.nodeID),
								zap.Error(err),
							)
							invalid = true
							break
						}
						if !c.vm.GetVerifyAuth() {
							continue
						}
						batchVerifier.Add(msg, tx.Auth)
					}

					// Don't wait for signatures if invalid
					//
					// TODO: stop job
					if invalid {
						batchVerifier.Done(nil)
						c.vm.Logger().Debug("dropping invalid tx gossip", zap.Stringer("nodeID", txw.nodeID))
						c.vm.metrics.gossipTxInvalid.Add(float64(len(txw.txs)))
						continue
					}

					// Wait for signature verification to finish
					batchVerifier.Done(nil)
					if err := job.Wait(); err != nil {
						c.vm.Logger().Debug(
							"received invalid gossip",
							zap.Stringer("peerID", txw.nodeID),
							zap.Error(err),
						)
						c.vm.metrics.gossipTxInvalid.Add(float64(len(txw.txs)))
						continue
					}

					// Submit txs
					errs := c.vm.Submit(ctx, false, txw.txs)
					now := time.Now().UnixMilli()
					for i, err := range errs {
						tx := txw.txs[i]
						if err == nil {
							c.vm.metrics.txTimeRemainingMempool.Observe(float64(tx.Expiry() - now))
							continue
						}
						c.vm.metrics.txGossipDropped.Inc()
						c.vm.Logger().Warn(
							"did not add incoming tx to mempool",
							zap.Stringer("peerID", txw.nodeID),
							zap.Stringer("txID", tx.ID()),
							zap.Error(err),
						)
					}
					c.vm.metrics.mempoolLen.Set(float64(c.vm.Mempool().Len(context.TODO())))
					c.vm.metrics.mempoolSize.Set(float64(c.vm.Mempool().Size(context.TODO())))
				case <-c.vm.stop:
					// If engine taking too long to process message, Shutdown will not
					// be called.
					c.vm.Logger().Info("stopping chunk manager")
					return nil
				}
			}
		})
	}
	for i := 0; i < c.vm.config.GetChunkStorageCores(); i++ {
		g.Go(func() error {
			for {
				select {
				case cw := <-c.incomingChunks:
					c.vm.metrics.gossipChunkBacklog.Dec()
					ctx := context.TODO()
					nodeID := cw.nodeID
					chunk := cw.chunk
					cert := cw.cert

					// Handle chunk verification
					if err := c.VerifyAndStoreChunk(ctx, nodeID, chunk); err != nil {
						c.vm.Logger().Warn("unable to verify and store chunk", zap.Stringer("nodeID", nodeID), zap.Stringer("chunkID", chunk.ID()), zap.Error(err))
						continue
					}

					// Handle certificate verification and enqueue chunk for auth
					if cert != nil {
						if err := c.VerifyAndTrackCertificate(ctx, cert); err != nil {
							c.vm.Logger().Warn("unable to verify and store certificate", zap.Stringer("nodeID", nodeID), zap.Stringer("chunkID", cert.Chunk), zap.Error(err))
							continue
						}
						c.auth.Add(chunk)
					} else {
						// In the case that we receive the cert for a chunk before the chunk itself,
						// we check to see if we have the cert.
						if _, ok := c.certs.Get(chunk.ID()); ok {
							c.auth.Add(chunk)
						}
					}

					// Sign chunk if validator
					_, epochHeight, err := c.getEpochInfo(ctx, chunk.Slot)
					if err != nil {
						c.vm.Logger().Warn("unable to determine chunk epoch", zap.Int64("slot", chunk.Slot), zap.Error(err))
						continue
					}
					amValidator, _, _, err := c.vm.proposerMonitor.IsValidator(ctx, epochHeight, c.vm.snowCtx.NodeID)
					if err != nil {
						c.vm.Logger().Warn("unable to determine if am validator", zap.Error(err))
						continue
					}
					if !amValidator {
						// We don't sign the chunk if we're not a validator
						continue
					}
					chunkSignature := &chain.ChunkSignature{
						Chunk: chunk.ID(),
						Slot:  chunk.Slot,
					}
					digest, err := chunkSignature.Digest()
					if err != nil {
						c.vm.Logger().Warn("cannot generate cert digest", zap.Stringer("nodeID", nodeID), zap.Error(err))
						continue
					}
					warpMessage, err := warp.NewUnsignedMessage(c.vm.snowCtx.NetworkID, c.vm.snowCtx.ChainID, digest)
					if err != nil {
						c.vm.Logger().Warn("unable to build warp message", zap.Error(err))
						continue
					}
					sig, err := c.vm.snowCtx.WarpSigner.Sign(warpMessage)
					if err != nil {
						c.vm.Logger().Warn("unable to sign chunk digest", zap.Error(err))
						continue
					}
					// We don't include the signer in the digest because we can't verify
					// the aggregate signature over the chunk if we do.
					chunkSignature.Signer = c.vm.snowCtx.PublicKey
					chunkSignature.Signature, err = bls.SignatureFromBytes(sig)
					if err != nil {
						c.vm.Logger().Warn("unable to parse signature", zap.Error(err))
						continue
					}
					// @todo the signature is sent to the chunk producer.
					c.PushSignature(ctx, nodeID, chunkSignature)
				case <-c.vm.stop:
					c.vm.Logger().Info("stopping chunk storage worker")
					return nil
				}
			}
		})
	}
	if err := g.Wait(); err != nil {
		c.vm.Logger().Error("chunk manager stopped with error", zap.Error(err))
	}
}

// Drop all chunks material that can no longer be included anymore (may have already been included).
//
// This functions returns an array of chunkIDs that can be used to delete unused chunks from persistent storage.
func (c *ChunkManager) SetBuildableMin(ctx context.Context, t int64) {
	removedBuilt := c.built.SetMin(t)
	expiredBuilt := 0
	for _, cid := range removedBuilt {
		if c.vm.IsSeenChunk(context.TODO(), cid) {
			continue
		}
		expiredBuilt++
		if cert, ok := c.certs.Get(cid); ok {
			c.vm.Logger().Warn(
				"dropping built chunk",
				zap.Stringer("chunkID", cid),
				zap.Int64("slot", cert.Slot),
				zap.Int("signers", cert.Signers.Len()),
			)
		}
	}
	c.vm.metrics.expiredBuiltChunks.Add(float64(expiredBuilt))
	removedCerts := c.certs.SetMin(ctx, t)
	expiredCerts := 0
	for _, cert := range removedCerts {
		if c.vm.IsSeenChunk(context.TODO(), cert.Chunk) {
			continue
		}
		expiredCerts++
	}
	c.vm.metrics.expiredCerts.Add(float64(expiredCerts))
}

// Remove chunks we included in a block to accurately account for unused chunks
func (c *ChunkManager) RemoveStored(chunk ids.ID) {
	c.stored.Remove(chunk)
}

// We keep track of all chunks we've stored, so we can delete them later.
func (c *ChunkManager) SetStoredMin(t int64) []*simpleChunkWrapper {
	return c.stored.SetMin(t)
}

func makeChunkMsg(chunk *chain.Chunk) ([]byte, error) {
	msg := make([]byte, 1+chunk.Size())
	msg[0] = chunkMsg
	chunkBytes, err := chunk.Marshal()
	if err != nil {
		return nil, err
	}
	copy(msg[1:], chunkBytes)
	return msg, nil
}

func (c *ChunkManager) PushChunk(ctx context.Context, chunk *chain.Chunk) {
	msg, err := makeChunkMsg(chunk)
	if err != nil {
		c.vm.Logger().Warn("failed to marshal chunk", zap.Error(err))
		return
	}
	_, epochHeight, err := c.getEpochInfo(ctx, chunk.Slot)
	if err != nil {
		c.vm.Logger().Warn("unable to determine chunk epoch", zap.Int64("t", chunk.Slot), zap.Error(err))
		return
	}
	validators, err := c.vm.proposerMonitor.GetValidatorSet(ctx, epochHeight, false)
	if err != nil {
		panic(err)
	}
	cw := &chunkWrapper{
		sent:       time.Now(),
		chunk:      chunk,
		signatures: make(map[string]*chain.ChunkSignature, len(validators)+1),
	}

	// Persist our own chunk
	if err := c.vm.StoreChunk(chunk); err != nil {
		panic(err)
	}

	// Sign our own chunk
	chunkSignature := &chain.ChunkSignature{
		Chunk: chunk.ID(),
		Slot:  chunk.Slot,
	}
	digest, err := chunkSignature.Digest()
	if err != nil {
		panic(err)
	}
	warpMessage, err := warp.NewUnsignedMessage(c.vm.snowCtx.NetworkID, c.vm.snowCtx.ChainID, digest)
	if err != nil {
		panic(err)
	}
	sig, err := c.vm.snowCtx.WarpSigner.Sign(warpMessage)
	if err != nil {
		panic(err)
	}
	// We don't include the signer in the digest because we can't verify
	// the aggregate signature over the chunk if we do.
	chunkSignature.Signer = c.vm.snowCtx.PublicKey
	chunkSignature.Signature, err = bls.SignatureFromBytes(sig)
	if err != nil {
		panic(err)
	}
	// TODO: can probably use uncompressed bytes here
	cw.signatures[string(bls.PublicKeyToCompressedBytes(chunkSignature.Signer))] = chunkSignature
	c.built.Add([]*chunkWrapper{cw})
	c.stored.Add(&simpleChunkWrapper{chunk: chunk.ID(), slot: chunk.Slot})

	// Send chunk to all validators
	//
	// TODO: consider changing to request (for signature)? -> would allow for a job poller style where we could keep sending?
	//
	// This would put some sort of latency requirement for other nodes to persist/sign the chunk (we should probably just let it flow
	// more loosely.
	c.appSender.SendAppGossip(ctx, common.SendConfig{NodeIDs: validators}, msg) // skips validators we aren't connected to
}

func makeChunkCertificateMsg(cert *chain.ChunkCertificate) ([]byte, error) {
	msg := make([]byte, 1+cert.Size())
	msg[0] = chunkCertificateMsg
	certBytes, err := cert.Marshal()
	if err != nil {
		return nil, err
	}
	copy(msg[1:], certBytes)
	return msg, nil
}

func (c *ChunkManager) PushChunkCertificate(ctx context.Context, cert *chain.ChunkCertificate) {
	msg, err := makeChunkCertificateMsg(cert)
	if err != nil {
		c.vm.Logger().Warn("failed to marshal chunk cert", zap.Error(err))
		return
	}
	_, epochHeight, err := c.getEpochInfo(ctx, cert.Slot)
	if err != nil {
		c.vm.Logger().Warn("unable to determine chunk epoch", zap.Int64("slot", cert.Slot), zap.Error(err))
		return
	}
	validators, err := c.vm.proposerMonitor.GetValidatorSet(ctx, epochHeight, false)
	if err != nil {
		panic(err)
	}

	go c.IssuePreConf(ctx, cert.ID(), epochHeight, cert.Signers)
	c.appSender.SendAppGossip(ctx, common.SendConfig{NodeIDs: validators}, msg) // skips validators we aren't connected to
}

func (c *ChunkManager) IssuePreConf(ctx context.Context, chunkID ids.ID, epochHeight uint64, bitSet set.Bits) {
	// currentHeight, _ := c.vm.GetLastAcceptedHeight()
	// nextProposer := c.vm.ProposerLookUp(ctx, currentHeight+1, epochHeight, 5)
	// // Construct certificate
	// canonicalValidators, _, err := c.vm.proposerMonitor.GetWarpValidatorSet(ctx, epochHeight)
	// if err != nil {
	// 	c.vm.Logger().Warn("cannot get canonical validator set", zap.Error(err))
	// }
	// var nextProposerIndex int
	// for i, vdr := range canonicalValidators {
	// 	if vdr.NodeIDs[0] == nextProposer {
	// 		nextProposerIndex = i
	// 		break
	// 	}
	// }
	// if !bitSet.Contains(nextProposerIndex) {
	// 	c.vm.Logger().Info("next proposer not in the signer set, no preconf given", zap.Stringer("chunkID", chunkID), zap.Uint64("height", epochHeight))
	// 	return
	// }
	// if err := c.vm.webSocketServer.GivePreconf(chunkID); err != nil {
	// 	c.vm.Logger().Warn("unable to give preconf", zap.Error(err))
	// }
	c.vm.webSocketServer.GivePreconf(chunkID)
}

func makeCertifiedChunkMsg(chunk *chain.Chunk, cert *chain.ChunkCertificate) ([]byte, error) {
	// We assume there is enough room to attach a cert to a chunk and still be under the network
	// size limit
	p := codec.NewWriter(1+codec.BytesLenSize(chunk.Size())+codec.BytesLenSize(cert.Size()), consts.NetworkSizeLimit)
	p.PackByte(chunkCertifiedMsg)
	c, err := chunk.Marshal()
	if err != nil {
		return nil, err
	}
	p.PackBytes(c)
	ct, err := cert.Marshal()
	if err != nil {
		return nil, err
	}
	p.PackBytes(ct)
	return p.Bytes(), p.Err()
}

func parseCertifiedChunkMsg(msg []byte, vm *VM) (*chain.Chunk, *chain.ChunkCertificate, error) {
	p := codec.NewReader(msg, consts.NetworkSizeLimit)
	p.UnpackByte()
	var chunkBytes []byte
	p.UnpackBytes(consts.NetworkSizeLimit, true, &chunkBytes)
	chunk, err := chain.UnmarshalChunk(chunkBytes, vm)
	if err != nil {
		return nil, nil, err
	}
	var certBytes []byte
	p.UnpackBytes(consts.NetworkSizeLimit, true, &certBytes)
	cert, err := chain.UnmarshalChunkCertificate(certBytes)
	if err != nil {
		return nil, nil, err
	}
	return chunk, cert, nil
}

func (c *ChunkManager) PushCertifiedChunk(ctx context.Context, chunk *chain.Chunk, cert *chain.ChunkCertificate) {
	// Get non-validators
	recipients := set.Set[ids.NodeID]{}
	_, epochHeight, err := c.getEpochInfo(ctx, cert.Slot)
	if err != nil {
		c.vm.Logger().Warn("unable to determine chunk epoch", zap.Int64("slot", cert.Slot), zap.Error(err))
		return
	}
	validators, err := c.vm.proposerMonitor.GetValidatorSet(ctx, epochHeight, false)
	if err != nil {
		panic(err)
	}
	c.connectedL.Lock()
	connected := c.connected.List()
	c.connectedL.Unlock()
	for _, recipient := range connected {
		if validators.Contains(recipient) || recipient == c.vm.snowCtx.NodeID {
			continue
		}
		recipients.Add(recipient)
		c.vm.metrics.optimisticCertifiedGossip.Inc()
	}

	// Push msg
	msg, err := makeCertifiedChunkMsg(chunk, cert)
	if err != nil {
		c.vm.Logger().Warn("failed to marshal certified chunk", zap.Error(err))
		return
	}
	c.appSender.SendAppGossip(ctx, common.SendConfig{NodeIDs: recipients}, msg)
}

// RestoreChunkCertificates re-inserts certs into the CertStore for inclusion. These chunks are sorted
// by the time they are first seen, so we always try to include certs that have been valid and around
// the longest first.
func (c *ChunkManager) RestoreChunkCertificates(ctx context.Context, certs []*chain.ChunkCertificate) {
	for _, cert := range certs {
		c.certs.Update(cert)
	}
}

func (c *ChunkManager) HandleTx(ctx context.Context, tx *chain.Transaction) {
	// TODO: drop if issued recently (recall we are not tracking in our mempool)?
	// -> We could track these txs in the mempool to prevent this issue.
	// -> Would then limit what we'd actually issue
	// -> This is more complex than it seems because if the node becomes a validator, it will then
	// start issuing chunks with invalid txs (not in correct partition).

	// Find transaction partition
	epoch, epochHeight, err := c.getEpochInfo(ctx, tx.Base.Timestamp)
	if err != nil {
		c.vm.Logger().Warn("cannot lookup epoch", zap.Error(err))
		return
	}
	partition, err := c.vm.proposerMonitor.AddressPartition(ctx, epoch, epochHeight, tx.Sponsor(), tx.Partition())
	if err != nil {
		c.vm.Logger().Warn("unable to compute address partition", zap.Error(err))
		return
	}

	// Add to mempool if we are the issuer
	if partition == c.vm.snowCtx.NodeID {
		c.vm.mempool.Add(ctx, []*chain.Transaction{tx})
		c.vm.Logger().Debug("adding tx to mempool", zap.Stringer("txID", tx.ID()))
		return
	}

	// Handle gossip addition
	txSize := tx.Size()
	txID := tx.ID()
	c.txL.Lock()
	var gossipable *txGossip
	gossip, ok := c.txNodes[partition]
	if ok {
		if gossip.size+txSize > gossipTxTargetSize {
			delete(c.txNodes, partition)
			gossip.sent = true
			gossipable = gossip
		} else {
			if _, ok := gossip.txs[txID]; !ok {
				gossip.txs[txID] = tx
				gossip.size += txSize
			}
			c.txL.Unlock()
			return
		}
	}
	gossip = &txGossip{
		nodeID: partition,
		txs:    make(map[ids.ID]*chain.Transaction, gossipTxPrealloc),
		expiry: time.Now().Add(gossipBatchWait),
		size:   consts.IntLen + txSize,
	}
	gossip.txs[txID] = tx
	c.txNodes[partition] = gossip
	c.txQueue.PushRight(gossip)
	c.txL.Unlock()

	// Send any gossip if exit early
	if gossipable == nil {
		return
	}
	c.sendTxGossip(ctx, gossipable)
}

func (c *ChunkManager) SignAnchorDigest(ctx context.Context, digest []byte) ([]byte, error) {
	now := time.Now().UnixMilli() - consts.ClockSkewAllowance
	r := c.vm.Rules(now)
	wm, err := warp.NewUnsignedMessage(r.NetworkID(), r.ChainID(), digest)
	if err != nil {
		return nil, err
	}

	return c.vm.Sign(wm)
}

// fields that are guaranteed populated:
// Slot, Txs, PriorityFeeReceiverAddr
func (c *ChunkManager) HandleAnchorChunk(ctx context.Context, anchor *chain.Anchor, slot int64, txs []*chain.Transaction, priorityFeeReceiverAddr codec.Address) error {
	chunk, err := chain.BuildChunkFromAnchor(ctx, c.vm, anchor, slot, txs, priorityFeeReceiverAddr)
	if err != nil {
		return err
	}

	// for gossipping & sig collecting
	c.auth.Add(chunk)
	c.PushChunk(ctx, chunk)

	return nil
}

func (c *ChunkManager) sendTxGossip(ctx context.Context, gossip *txGossip) {
	txs := maps.Values(gossip.txs)
	txBytes, err := chain.MarshalTxs(txs)
	if err != nil {
		panic(err)
	}
	msg := make([]byte, 1+len(txBytes))
	msg[0] = txMsg
	copy(msg[1:], txBytes)
	c.appSender.SendAppGossip(ctx, common.SendConfig{NodeIDs: set.Of(gossip.nodeID)}, msg) // skips validators we aren't connected to
	c.vm.Logger().Debug(
		"sending txs to partition",
		zap.Int("txs", len(txs)),
		zap.Stringer("partition", gossip.nodeID),
		zap.Int("size", len(msg)),
	)
	c.vm.RecordTxsGossiped(len(txs))
}

// This function should be spawned in a goroutine because it blocks
func (c *ChunkManager) RequestChunks(block uint64, certs []*chain.ChunkCertificate, chunks chan *chain.Chunk) {
	// Ensure only one fetch is running at a time (all bandwidth should be allocated towards fetching chunks for next block)
	myWaiter := make(chan struct{})
	c.waiterL.Lock()
	waiter := c.waiter
	c.waiter = myWaiter
	c.waiterL.Unlock()

	// Kickoff job async
	go func() {
		if waiter != nil { // nil at first
			select {
			case <-waiter:
			case <-c.vm.stop:
				return
			}
		}

		// Kickoff fetch
		fetchStart := time.Now()
		workers := min(len(certs), c.vm.config.GetMissingChunkFetchers())
		f := opool.New(workers, len(certs))
		for _, rcert := range certs {
			cert := rcert
			f.Go(func() (func(), error) {
				// Look for chunk
				chunk, err := c.vm.GetChunk(cert.Slot, cert.Chunk)
				if chunk != nil {
					c.auth.Add(chunk)
					return func() { chunks <- chunk }, nil
				}
				c.vm.Logger().Debug("missing chunk", zap.Stringer("chunkID", cert.Chunk), zap.Int64("slot", cert.Slot), zap.Error(err))

				// Look for chunk epoch
				_, epochHeight, err := c.getEpochInfo(context.TODO(), cert.Slot)
				if err != nil {
					c.vm.Logger().Warn("cannot lookup epoch", zap.Error(err))
					return nil, err
				}

				// Fetch missing chunk
				attempts := 0
				for {
					// Check to see if we received chunk while we were fetching
					if attempts > 0 {
						chunk, _ := c.vm.GetChunk(cert.Slot, cert.Chunk)
						if chunk != nil {
							c.vm.metrics.uselessFetchChunkAttempts.Inc()
							c.vm.Logger().Debug("received chunk while fetching", zap.Stringer("chunkID", cert.Chunk))
							c.auth.Add(chunk)
							return func() { chunks <- chunk }, nil
						}
					}

					// Make request
					c.vm.metrics.fetchChunkAttempts.Add(1)
					c.vm.Logger().Debug("fetching missing chunk", zap.Int64("slot", cert.Slot), zap.Stringer("chunkID", cert.Chunk), zap.Int("previous attempts", attempts))
					attempts++

					bytesChan := make(chan []byte, 1)
					c.callbacksL.Lock()
					requestID := c.requestID
					c.callbacks[requestID] = func(response []byte) {
						bytesChan <- response
						close(bytesChan)
					}
					c.requestID++
					c.callbacksL.Unlock()
					var validator ids.NodeID
					for {
						// Chunk should be sent to all validators, so we can just pick a random one
						//
						// TODO: consider using cert to select validators?
						randomValidator, err := c.vm.proposerMonitor.RandomValidator(context.Background(), epochHeight)
						if err != nil {
							return nil, err
						}
						c.connectedL.Lock()
						contains := c.connected.Contains(randomValidator)
						c.connectedL.Unlock()
						if contains {
							validator = randomValidator
							break
						}
						c.vm.Logger().Warn("skipping disconnected validator", zap.Stringer("nodeID", randomValidator))
						// TODO: put some time delay here to avoid spinning when disconnected?
					}
					request := make([]byte, 1+consts.Uint64Len+ids.IDLen)
					request[0] = chunkReq
					binary.BigEndian.PutUint64(request[1:], uint64(cert.Slot))
					copy(request[1+consts.Uint64Len:], cert.Chunk[:])
					if err := c.appSender.SendAppRequest(context.Background(), set.Of(validator), requestID, request); err != nil {
						c.vm.Logger().Warn("failed to send chunk request", zap.Error(err))
						continue
					}

					// Wait for reponse or exit
					var bytes []byte
					select {
					case bytes = <-bytesChan:
					case <-c.vm.stop:
						return nil, errors.New("stopping")
					}
					if len(bytes) == 0 {
						c.vm.Logger().Warn("failed to fetch chunk", zap.Stringer("chunkID", cert.Chunk), zap.Stringer("nodeID", validator))
						continue
					}

					// Handle response
					//
					// Note: we may have received the chunk we are fetching from push gossip
					// while fetching for it.
					chunk, err := chain.UnmarshalChunk(bytes, c.vm)
					if err != nil {
						c.vm.Logger().Warn("failed to unmarshal chunk", zap.Error(err))
						continue
					}
					if chunk.ID() != cert.Chunk {
						c.vm.Logger().Warn("unexpected chunk", zap.Stringer("chunkID", chunk.ID()), zap.Stringer("expectedChunkID", cert.Chunk))
						continue
					}
					// Check if received during push gossip
					if c.stored.Has(chunk.ID()) {
						c.auth.Add(chunk)
						c.vm.metrics.uselessFetchChunkAttempts.Inc()
						return func() { chunks <- chunk }, nil
					}
					if err := c.vm.StoreChunk(chunk); err != nil {
						return nil, err
					}
					c.stored.Add(&simpleChunkWrapper{chunk: chunk.ID(), slot: cert.Slot})
					c.auth.Add(chunk)
					return func() { chunks <- chunk }, nil
				}
			})
		}
		ferr := f.Wait()
		close(chunks) // We always close chunks, so the execution engine can handle this issue and shutdown
		if ferr != nil {
			// This is a FATAL because exiting here will cause the execution engine to hang
			c.vm.Logger().Fatal("failed to fetch chunks", zap.Error(ferr))
			return
		}
		c.vm.Logger().Info("finished fetching chunks", zap.Uint64("height", block), zap.Duration("t", time.Since(fetchStart)))
		c.vm.metrics.fetchMissingChunks.Observe(float64(time.Since(fetchStart)))

		// Invoke next waiter
		close(myWaiter)
	}()
}

func (c *ChunkManager) Done() {
	<-c.done
}

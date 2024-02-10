package vm

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/validators"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/cache"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/consts"
	"github.com/ava-labs/hypersdk/eheap"
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/list"
	"github.com/ava-labs/hypersdk/utils"
	"github.com/sourcegraph/conc/stream"
	"go.uber.org/zap"
)

const (
	chunkMsg            uint8 = 0x0
	chunkSignatureMsg   uint8 = 0x1
	chunkCertificateMsg uint8 = 0x2
	txMsg               uint8 = 0x3

	chunkReq         uint8 = 0x0
	filteredChunkReq uint8 = 0x1

	minWeightNumerator = 67
	weightDenominator  = 100
)

type chunkWrapper struct {
	l sync.Mutex

	chunk      *chain.Chunk
	signatures map[ids.ID]*chain.ChunkSignature
}

func (cw *chunkWrapper) ID() ids.ID {
	id, err := cw.chunk.ID()
	if err != nil {
		panic(err)
	}
	return id
}

func (cw *chunkWrapper) Expiry() int64 {
	return cw.chunk.Slot
}

// Store fifo chunks for building
type CertStore struct {
	l sync.Mutex

	queue *list.List[*chain.ChunkCertificate]
	eh    *eheap.ExpiryHeap[*list.Element[*chain.ChunkCertificate]]
}

func NewCertStore() *CertStore {
	return &CertStore{
		queue: &list.List[*chain.ChunkCertificate]{},
		eh:    eheap.New[*list.Element[*chain.ChunkCertificate]](64), // TODO: add a config
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

	elem, ok := c.eh.Get(cert.ID())
	if !ok {
		elem = c.queue.PushBack(cert)
	} else {
		// If the existing certificate has more signers than the
		// new certificate, don't update.
		if elem.Value().Signers.Len() > cert.Signers.Len() {
			return
		}
	}
	c.eh.Update(elem)
}

// Called when a block is accepted with valid certs.
func (c *CertStore) SetMin(ctx context.Context, t int64) {
	c.l.Lock()
	defer c.l.Unlock()

	removedElems := c.eh.SetMin(t)
	for _, remove := range removedElems {
		c.queue.Remove(remove)
	}
}

// Pop removes and returns the highest valued item in m.eh.
func (c *CertStore) Pop(ctx context.Context) (*chain.ChunkCertificate, bool) { // O(log N)
	c.l.Lock()
	defer c.l.Unlock()

	first := c.queue.First()
	if first == nil {
		return nil, false
	}
	v := c.queue.Remove(first)
	c.eh.Remove(v.ID())
	return v, true
}

// TODO: move to standalone package
type ChunkManager struct {
	vm   *VM
	done chan struct{}

	appSender common.AppSender

	txs *cache.FIFO[ids.ID, any]

	built *emap.EMap[*chunkWrapper]

	certs *CertStore

	// Ensures that only one request job is running at a time
	waiterL sync.Mutex
	waiter  chan struct{}

	// Handles concurrent fetching of chunks
	callbacksL sync.Mutex
	requestID  uint32
	callbacks  map[uint32]func([]byte)

	// connected includes all connected nodes, not just those that are validators (we use
	// this for sending txs/requesting chunks from only connected nodes)
	connected set.Set[ids.NodeID]
}

func NewChunkManager(vm *VM) *ChunkManager {
	cache, err := cache.NewFIFO[ids.ID, any](16384)
	if err != nil {
		panic(err)
	}
	return &ChunkManager{
		vm:   vm,
		done: make(chan struct{}),

		txs: cache,

		built: emap.NewEMap[*chunkWrapper](),
		certs: NewCertStore(),

		callbacks: make(map[uint32]func([]byte)),

		connected: set.NewSet[ids.NodeID](64), // TODO: make a const
	}
}

func (c *ChunkManager) getEpochHeight(ctx context.Context, t int64) (uint64, error) {
	r := c.vm.Rules(time.Now().UnixMilli())
	epoch := utils.Epoch(t, r.GetEpochDuration())
	_, heights, err := c.vm.Engine().GetEpochHeights(ctx, []uint64{epoch})
	if err != nil {
		return 0, err
	}
	if heights[0] == nil {
		return 0, errors.New("epoch is not yet set")
	}
	return *heights[0], nil
}

func (c *ChunkManager) Connected(_ context.Context, nodeID ids.NodeID, _ *version.Application) error {
	c.connected.Add(nodeID)
	return nil
}

func (c *ChunkManager) Disconnected(_ context.Context, nodeID ids.NodeID) error {
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
	c.appSender.SendAppGossipSpecific(ctx, set.Of(nodeID), msg) // skips validators we aren't connected to
}

func (c *ChunkManager) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	if len(msg) == 0 {
		c.vm.Logger().Warn("dropping empty message", zap.Stringer("nodeID", nodeID))
		return nil
	}
	switch msg[0] {
	case chunkMsg:
		chunk, err := chain.UnmarshalChunk(msg[1:], c.vm)
		if err != nil {
			c.vm.Logger().Warn("unable to unmarshal chunk", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}

		// Check that producer is the sender
		if chunk.Producer != nodeID {
			c.vm.Logger().Warn("dropping chunk gossip that isn't from producer", zap.Stringer("nodeID", nodeID))
			return nil
		}

		// Determine if chunk producer is a validator and that their key is valid
		epochHeight, err := c.getEpochHeight(ctx, chunk.Slot)
		if err != nil {
			c.vm.Logger().Warn("unable to determine chunk epoch", zap.Int64("slot", chunk.Slot), zap.Error(err))
			return nil
		}
		isValidator, signerKey, _, err := c.vm.proposerMonitor.IsValidator(ctx, epochHeight, chunk.Producer)
		if err != nil {
			c.vm.Logger().Warn("unable to determine if producer is validator", zap.Stringer("producer", chunk.Producer), zap.Error(err))
			return nil
		}
		if !isValidator {
			c.vm.Logger().Warn("dropping chunk gossip from non-validator", zap.Stringer("nodeID", nodeID))
			return nil
		}
		if signerKey == nil || !bytes.Equal(bls.PublicKeyToBytes(chunk.Signer), bls.PublicKeyToBytes(signerKey)) {
			c.vm.Logger().Warn("dropping validator signed chunk with wrong key", zap.Stringer("nodeID", nodeID))
			return nil
		}

		// Verify signature of chunk
		if !chunk.VerifySignature(c.vm.snowCtx.NetworkID, c.vm.snowCtx.ChainID) {
			dig, err := chunk.Digest()
			if err != nil {
				panic(err)
			}
			c.vm.Logger().Warn(
				"dropping chunk with invalid signature",
				zap.Stringer("nodeID", nodeID),
				zap.Uint32("networkID", c.vm.snowCtx.NetworkID),
				zap.Stringer("chainID", c.vm.snowCtx.ChainID),
				zap.String("digest", hex.EncodeToString(dig)),
				zap.String("signer", hex.EncodeToString(bls.PublicKeyToBytes(chunk.Signer))),
				zap.String("signature", hex.EncodeToString(bls.SignatureToBytes(chunk.Signature))),
			)
			return nil
		}

		// TODO: only store 1 chunk per slot per validator
		// TODO: warn if chunk is dropped for a conflict during fetching (same producer, slot, different chunkID)

		// Persist chunk to disk (delete if not used in time but need to store to protect
		// against shutdown risk across network -> chunk may no longer be accessible after included
		// in referenced certificate)
		if err := c.vm.StoreChunk(chunk); err != nil {
			c.vm.Logger().Warn("unable to persist chunk to disk", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}

		// Sign chunk
		cid, err := chunk.ID()
		if err != nil {
			c.vm.Logger().Warn("cannot generate id", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}
		chunkSignature := &chain.ChunkSignature{
			Chunk: cid,
			Slot:  chunk.Slot,
		}
		digest, err := chunkSignature.Digest()
		if err != nil {
			c.vm.Logger().Warn("cannot generate cert digest", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}
		warpMessage, err := warp.NewUnsignedMessage(c.vm.snowCtx.NetworkID, c.vm.snowCtx.ChainID, digest)
		if err != nil {
			c.vm.Logger().Warn("unable to build warp message", zap.Error(err))
			return nil
		}
		sig, err := c.vm.snowCtx.WarpSigner.Sign(warpMessage)
		if err != nil {
			c.vm.Logger().Warn("unable to sign chunk digest", zap.Error(err))
			return nil
		}
		// We don't include the signer in the digest because we can't verify
		// the aggregate signature over the chunk if we do.
		chunkSignature.Signer = c.vm.snowCtx.PublicKey
		chunkSignature.Signature, err = bls.SignatureFromBytes(sig)
		if err != nil {
			c.vm.Logger().Warn("unable to parse signature", zap.Error(err))
			return nil
		}
		c.PushSignature(ctx, nodeID, chunkSignature)
	case chunkSignatureMsg:
		chunkSignature, err := chain.UnmarshalChunkSignature(msg[1:])
		if err != nil {
			c.vm.Logger().Warn("dropping chunk gossip from non-validator", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}

		// Check if we broadcast this chunk
		cw, ok := c.built.Get(chunkSignature.Chunk)
		if !ok || cw.chunk.Slot != chunkSignature.Slot {
			c.vm.Logger().Warn("dropping useless chunk signature", zap.Stringer("nodeID", nodeID), zap.Stringer("chunkID", chunkSignature.Chunk))
			return nil
		}

		// Determine if chunk signer is a validator and that their key is valid
		epochHeight, err := c.getEpochHeight(ctx, chunkSignature.Slot)
		if err != nil {
			c.vm.Logger().Warn("unable to determine chunk epoch", zap.Int64("slot", chunkSignature.Slot))
			return nil
		}
		isValidator, signerKey, _, err := c.vm.proposerMonitor.IsValidator(ctx, epochHeight, nodeID)
		if err != nil {
			c.vm.Logger().Warn("unable to determine if signer is validator", zap.Stringer("signer", nodeID), zap.Error(err))
			return nil
		}
		if !isValidator {
			c.vm.Logger().Warn("dropping chunk signature from non-validator", zap.Stringer("nodeID", nodeID))
			return nil
		}
		if signerKey == nil || !bytes.Equal(bls.PublicKeyToBytes(chunkSignature.Signer), bls.PublicKeyToBytes(signerKey)) {
			c.vm.Logger().Warn("dropping validator signed chunk with wrong key", zap.Stringer("nodeID", nodeID))
			return nil
		}

		// Verify signature
		if !chunkSignature.VerifySignature(c.vm.snowCtx.NetworkID, c.vm.snowCtx.ChainID) {
			c.vm.Logger().Warn("dropping chunk signature with invalid signature", zap.Stringer("nodeID", nodeID))
			return nil
		}

		// Store signature for chunk
		cw.l.Lock()
		cw.signatures[utils.ToID(bls.PublicKeyToBytes(chunkSignature.Signer))] = chunkSignature // canonical validator set requires fetching signature by bls public key

		// Count pending weight
		//
		// TODO: add safe math
		// TODO: handle changing validator sets
		var (
			weight      uint64 = 0
			totalWeight uint64 = 0
		)
		if err := c.vm.proposerMonitor.IterateValidators(ctx, epochHeight, func(vdr ids.NodeID, out *validators.GetValidatorOutput) {
			totalWeight += out.Weight
			if out.PublicKey == nil {
				return
			}
			k := utils.ToID(bls.PublicKeyToBytes(out.PublicKey))
			if _, ok := cw.signatures[k]; ok {
				weight += out.Weight
			}
		}); err != nil {
			panic(err)
		}
		cw.l.Unlock()

		// Check if weight is sufficient
		if err := warp.VerifyWeight(weight, totalWeight, c.vm.config.GetMinimumCertificateBroadcastNumerator(), weightDenominator); err != nil {
			c.vm.Logger().Warn("chunk does not have sufficient weight to crete certificate", zap.Stringer("chunkID", chunkSignature.Chunk), zap.Error(err))
			return nil
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
			sig, ok := cw.signatures[utils.ToID(bls.PublicKeyToBytes(vdr.PublicKey))]
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
		c.certs.Update(cert)

		// Broadcast certificate
		//
		// Each time we get more signatures, we'll broadcast a new
		// certificate
		c.PushChunkCertificate(ctx, cert)
		c.vm.Logger().Info(
			"constructed chunk certificate",
			zap.Uint64("Pheight", epochHeight),
			zap.Stringer("chunkID", chunkSignature.Chunk),
			zap.Uint64("weight", weight),
			zap.Uint64("totalWeight", totalWeight),
		)
	case chunkCertificateMsg:
		cert, err := chain.UnmarshalChunkCertificate(msg[1:])
		if err != nil {
			c.vm.Logger().Warn("dropping chunk gossip from non-validator", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}

		// Determine epoch for certificate
		epochHeight, err := c.getEpochHeight(ctx, cert.Slot)
		if err != nil {
			c.vm.Logger().Warn("unable to determine certificate epoch", zap.Int64("slot", cert.Slot))
			return nil
		}

		// Verify certificate using the epoch validator set
		validators, weight, err := c.vm.proposerMonitor.GetWarpValidatorSet(ctx, epochHeight)
		if err != nil {
			c.vm.Logger().Warn("cannot get canonical validator set", zap.Error(err))
			return nil
		}
		filteredVdrs, err := warp.FilterValidators(cert.Signers, validators)
		if err != nil {
			return err
		}
		filteredWeight, err := warp.SumWeight(filteredVdrs)
		if err != nil {
			return err
		}
		if err := warp.VerifyWeight(filteredWeight, weight, minWeightNumerator, weightDenominator); err != nil {
			c.vm.Logger().Warn(
				"dropping invalid certificate",
				zap.Uint64("Pheight", epochHeight),
				zap.Stringer("nodeID", nodeID),
				zap.Stringer("chunkID", cert.Chunk),
				zap.Error(err),
			)
			return err
		}
		aggrPubKey, err := warp.AggregatePublicKeys(filteredVdrs)
		if err != nil {
			return err
		}
		if !cert.VerifySignature(c.vm.snowCtx.NetworkID, c.vm.snowCtx.ChainID, aggrPubKey) {
			c.vm.Logger().Warn(
				"dropping invalid certificate",
				zap.Uint64("Pheight", epochHeight),
				zap.Stringer("nodeID", nodeID),
				zap.Stringer("chunkID", cert.Chunk),
				zap.Binary("bitset", cert.Signers.Bytes()),
				zap.String("aggrPubKey", hex.EncodeToString(bls.PublicKeyToBytes(aggrPubKey))),
				zap.String("signature", hex.EncodeToString(bls.SignatureToBytes(cert.Signature))),
				zap.Error(errors.New("invalid signature")),
			)
			return nil
		}
		c.vm.Logger().Info(
			"verified chunk certificate",
			zap.Uint64("Pheight", epochHeight),
			zap.Stringer("nodeID", nodeID),
			zap.Stringer("chunkID", cert.Chunk),
			zap.String("aggrPubKey", hex.EncodeToString(bls.PublicKeyToBytes(aggrPubKey))),
			zap.String("signature", hex.EncodeToString(bls.SignatureToBytes(cert.Signature))),
			zap.Uint64("weight", filteredWeight),
			zap.Uint64("totalWeight", weight),
		)
		// If we don't have the chunk, we wait to fetch it until the certificate is included in an accepted block.

		// TODO: if this certificate conflicts with a chunk we signed, post the conflict (slashable fault)

		// Store chunk certificate for building
		c.certs.Update(cert)
	case txMsg:
		_, txs, err := chain.UnmarshalTxs(msg[1:], 1, c.vm.actionRegistry, c.vm.authRegistry)
		if err != nil {
			c.vm.Logger().Warn("dropping invalid tx gossip from non-validator", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}
		tx := txs[0]

		// Check that we are the partition for the tx
		epochHeight, err := c.getEpochHeight(ctx, tx.Base.Timestamp)
		if err != nil {
			c.vm.Logger().Warn("unable to determine tx epoch", zap.Int64("t", tx.Base.Timestamp))
			return nil
		}
		partition, err := c.vm.proposerMonitor.AddressPartition(ctx, epochHeight, tx.Sponsor())
		if err != nil {
			c.vm.Logger().Warn("unable to compute address partition", zap.Error(err))
			return nil
		}
		if partition != c.vm.snowCtx.NodeID {
			c.vm.Logger().Warn("dropping tx gossip from non-partition", zap.Stringer("nodeID", nodeID))
			return nil
		}

		// Submit txs
		c.vm.Submit(ctx, true, txs)
	default:
		c.vm.Logger().Warn("dropping unknown message type", zap.Stringer("nodeID", nodeID))
	}
	return nil
}

// TODO: support returning an error to the sender instead of just timing out (will make
// miss handling MUCH faster)
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
		chunk, err := c.vm.GetChunk(slot, id)
		if err != nil {
			c.vm.Logger().Warn(
				"unable to fetch chunk",
				zap.Stringer("nodeID", nodeID),
				zap.Int64("slot", slot),
				zap.Stringer("chunkID", id),
				zap.Error(err),
			)
			return nil
		}
		chunkBytes, err := chunk.Marshal()
		if err != nil {
			panic(err)
		}
		c.appSender.SendAppResponse(ctx, nodeID, requestID, chunkBytes)
	case filteredChunkReq:
		rid := request[1:]
		if len(rid) != ids.IDLen {
			c.vm.Logger().Warn("dropping invalid request", zap.Stringer("nodeID", nodeID))
			return nil
		}
		id := ids.ID(rid)
		chunk, err := c.vm.GetFilteredChunk(id)
		if err != nil {
			c.vm.Logger().Warn(
				"unable to fetch filtered chunk",
				zap.Stringer("nodeID", nodeID),
				zap.Stringer("chunkID", id),
				zap.Error(err),
			)
			return nil
		}
		chunkBytes, err := chunk.Marshal()
		if err != nil {
			panic(err)
		}
		c.appSender.SendAppResponse(ctx, nodeID, requestID, chunkBytes)
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

func (c *ChunkManager) Run(appSender common.AppSender) {
	c.appSender = appSender

	c.vm.Logger().Info("starting chunk manager")
	defer close(c.done)

	t := time.NewTicker(c.vm.config.GetBuildFrequency())
	defer t.Stop()
	for {
		select {
		case <-t.C:
			if !c.vm.isReady() {
				c.vm.Logger().Info("skipping chunk loop because vm isn't ready")
				continue
			}

			// TODO: move this simple time trigger elsewhere
			select {
			case c.vm.EngineChan() <- common.PendingTxs:
				c.vm.Logger().Info("sent message to build")
			default:
				c.vm.Logger().Info("already sent message to build")
			}

			chunk, err := chain.BuildChunk(context.TODO(), c.vm)
			if err != nil {
				c.vm.Logger().Warn("unable to build chunk", zap.Error(err))
				continue
			}
			c.PushChunk(context.TODO(), chunk)
			cid, err := chunk.ID()
			if err != nil {
				// TODO: pre-calc ID
				panic(err)
			}
			c.vm.Logger().Info(
				"built chunk",
				zap.Stringer("id", cid),
				zap.Int("txs", len(chunk.Txs)),
			)
		case <-c.vm.stop:
			c.vm.Logger().Info("stopping chunk manager")
			return
		}
	}
}

// Drop all chunks material that can no longer be included anymore (may have already been included).
func (c *ChunkManager) SetMin(ctx context.Context, t int64) {
	c.built.SetMin(t) // discarded chunks will be cleaned up async
	c.certs.SetMin(ctx, t)
}

func (c *ChunkManager) PushChunk(ctx context.Context, chunk *chain.Chunk) {
	msg := make([]byte, 1+chunk.Size())
	msg[0] = chunkMsg
	chunkBytes, err := chunk.Marshal()
	if err != nil {
		c.vm.Logger().Warn("failed to marshal chunk", zap.Error(err))
		return
	}
	copy(msg[1:], chunkBytes)
	epochHeight, err := c.getEpochHeight(ctx, chunk.Slot)
	if err != nil {
		c.vm.Logger().Warn("unable to determine chunk epoch", zap.Int64("t", chunk.Slot), zap.Error(err))
		return
	}
	validators, err := c.vm.proposerMonitor.GetValidatorSet(ctx, epochHeight, false)
	if err != nil {
		panic(err)
	}
	cw := &chunkWrapper{
		chunk:      chunk,
		signatures: make(map[ids.ID]*chain.ChunkSignature, len(validators)+1),
	}

	// Persist our own chunk
	if err := c.vm.StoreChunk(chunk); err != nil {
		panic(err)
	}

	// Sign our own chunk
	cid, _ := chunk.ID()
	chunkSignature := &chain.ChunkSignature{
		Chunk: cid,
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
	cw.signatures[utils.ToID(bls.PublicKeyToBytes(chunkSignature.Signer))] = chunkSignature
	c.built.Add([]*chunkWrapper{cw})

	// Send chunk to all validators
	//
	// TODO: consider changing to request (for signature)? -> would allow for a job poller style where we could keep sending?
	//
	// This would put some sort of latency requirement for other nodes to persist/sign the chunk (we should probably just let it flow
	// more loosely.
	c.appSender.SendAppGossipSpecific(ctx, validators, msg) // skips validators we aren't connected to
}

func (c *ChunkManager) PushChunkCertificate(ctx context.Context, cert *chain.ChunkCertificate) {
	msg := make([]byte, 1+cert.Size())
	msg[0] = chunkCertificateMsg
	certBytes, err := cert.Marshal()
	if err != nil {
		c.vm.Logger().Warn("failed to marshal chunk", zap.Error(err))
		return
	}
	copy(msg[1:], certBytes)
	epochHeight, err := c.getEpochHeight(ctx, cert.Slot)
	if err != nil {
		c.vm.Logger().Warn("unable to determine chunk epoch", zap.Int64("slot", cert.Slot), zap.Error(err))
		return
	}
	validators, err := c.vm.proposerMonitor.GetValidatorSet(ctx, epochHeight, false)
	if err != nil {
		panic(err)
	}
	c.appSender.SendAppGossipSpecific(ctx, validators, msg) // skips validators we aren't connected to
}

func (c *ChunkManager) NextChunkCertificate(ctx context.Context) (*chain.ChunkCertificate, bool) {
	return c.certs.Pop(ctx)
}

// TODO: ensure they are at front?
func (c *ChunkManager) RestoreChunkCertificates(ctx context.Context, certs []*chain.ChunkCertificate) {
	for _, cert := range certs {
		c.certs.Update(cert)
	}
}

func (c *ChunkManager) HandleTx(ctx context.Context, tx *chain.Transaction) {
	// Check if issued recently
	if _, ok := c.txs.Get(tx.ID()); ok {
		return
	}
	c.txs.Put(tx.ID(), nil)

	// Find transaction partition
	epochHeight, err := c.getEpochHeight(ctx, tx.Base.Timestamp)
	if err != nil {
		c.vm.Logger().Warn("cannot lookup epoch", zap.Error(err))
		return
	}
	partition, err := c.vm.proposerMonitor.AddressPartition(ctx, epochHeight, tx.Sponsor())
	if err != nil {
		c.vm.Logger().Warn("unable to compute address partition", zap.Error(err))
		return
	}

	// Add to mempool if we are the issuer
	if partition == c.vm.snowCtx.NodeID {
		c.vm.mempool.Add(ctx, []*chain.Transaction{tx})
		return
	}

	// If we are not the issuer, send to the correct issuer
	//
	// TODO: batch tx issuance
	msg, err := chain.MarshalTxs([]*chain.Transaction{tx})
	if err != nil {
		panic(err)
	}
	c.appSender.SendAppGossipSpecific(ctx, set.Of(partition), msg)
}

// This function should be spawned in a goroutine because it blocks
func (c *ChunkManager) RequestChunks(certs []*chain.ChunkCertificate, chunks chan *chain.Chunk) {
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
		fetchStream := stream.New()
		fetchStream.WithMaxGoroutines(2) // TODO: use config
		for _, rcert := range certs {
			cert := rcert
			fetchStream.Go(func() stream.Callback {
				// Look for chunk
				chunk, err := c.vm.GetChunk(cert.Slot, cert.Chunk)
				if err == nil {
					return func() { chunks <- chunk }
				}

				// Fetch missing chunk
				attempts := 0
				for {
					c.vm.Logger().Warn("fetching missing chunk", zap.Int64("slot", cert.Slot), zap.Stringer("chunkID", cert.Chunk), zap.Int("previous attempts", attempts))

					// Look for chunk epoch
					epochHeight, err := c.getEpochHeight(context.TODO(), cert.Slot)
					if err != nil {
						c.vm.Logger().Warn("cannot lookup epoch", zap.Error(err))
						continue
					}

					// Make request
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
							panic(err)
						}
						if c.connected.Contains(randomValidator) {
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
						return func() { chunks <- nil }
					}
					if len(bytes) == 0 {
						continue
					}

					// Handle response
					chunk, err := chain.UnmarshalChunk(bytes, c.vm)
					if err != nil {
						c.vm.Logger().Warn("failed to unmarshal chunk", zap.Error(err))
						continue
					}
					chunkID, err := chunk.ID()
					if err != nil {
						panic(err)
					}
					if chunkID != cert.Chunk {
						c.vm.Logger().Warn("unexpected chunk", zap.Stringer("chunkID", chunkID), zap.Stringer("expectedChunkID", cert.Chunk))
						continue
					}
					if err := c.vm.StoreChunk(chunk); err != nil {
						panic(err)
					}
					return func() { chunks <- chunk }
				}
			})
		}
		fetchStream.Wait()
		close(chunks)

		// Invoke next waiter
		close(myWaiter)
	}()
}

func (c *ChunkManager) Done() {
	<-c.done
}

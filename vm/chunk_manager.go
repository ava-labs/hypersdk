package vm

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/crypto/bls"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/vms/platformvm/warp"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/eheap"
	"github.com/ava-labs/hypersdk/emap"
	"github.com/ava-labs/hypersdk/list"
	"github.com/ava-labs/hypersdk/utils"
	"go.uber.org/zap"
)

const (
	chunkMsg            uint8 = 0x0
	chunkSignatureMsg   uint8 = 0x1
	chunkCertificateMsg uint8 = 0x2
)

type chunkWrapper struct {
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

// TODO: emap of chunks (delete IDs from disk that aren't included on-chain), don't remove when block accepted at timestamp (rather do later after execution final)
type ChunkStore struct {
}

// TODO: store FIFO chunk certs
type CertStore struct {
	l *sync.Mutex

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
func (c *CertStore) Add(cert *chain.ChunkCertificate) {
	c.l.Lock()
	defer c.l.Unlock()

	if c.eh.Has(cert.ID()) {
		return
	}
	elem := c.queue.PushBack(cert)
	c.eh.Add(elem)
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
	vm *VM

	appSender common.AppSender

	built  *emap.EMap[*chunkWrapper]
	chunks *ChunkStore
	certs  *CertStore

	// connected includes all connected nodes, not just those that are validators (we use
	// this for requesting chunks from only connected nodes)
	connected set.Set[ids.NodeID]
}

func NewChunkManager(vm *VM) *ChunkManager {
	return &ChunkManager{
		vm: vm,

		built: emap.NewEMap[*chunkWrapper](),
		certs: NewCertStore(),

		connected: set.NewSet[ids.NodeID](64), // TODO: make a const
	}
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
	// Check that sender is a validator
	ok, pk, _, err := c.vm.proposerMonitor.IsValidator(ctx, nodeID)
	if err != nil {
		c.vm.Logger().Warn("unable to determine if node is a validator", zap.Error(err))
		return nil
	}
	if !ok {
		c.vm.Logger().Warn("dropping chunk gossip from non-validator", zap.Stringer("nodeID", nodeID))
		return nil
	}
	if len(msg) == 0 {
		c.vm.Logger().Warn("dropping empty message", zap.Stringer("nodeID", nodeID))
		return nil
	}
	switch msg[0] {
	case chunkMsg:
		chunk, err := chain.UnmarshalChunk(msg[1:], c.vm)
		if err != nil {
			c.vm.Logger().Warn("dropping chunk gossip from non-validator", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}

		// TODO: check validity (verify chunk signature)
		// TODO: only store 1 chunk per slot per validator
		if chunk.Producer != nodeID {
			c.vm.Logger().Warn("dropping chunk gossip that isn't from producer", zap.Stringer("nodeID", nodeID))
			return nil
		}
		// TODO: ensure signer of chunk is associated with NodeID
		if !chunk.VerifySignature(c.vm.snowCtx.NetworkID, c.vm.snowCtx.ChainID) {
			c.vm.Logger().Warn("dropping chunk with invalid signature", zap.Stringer("nodeID", nodeID))
			return nil
		}

		// Persist chunk to disk (delete if not used in time but need to store to protect
		// against shutdown risk across network -> chunk may no longer be accessible after included
		// in referenced certificate)
		if err := c.vm.StoreChunk(chunk); err != nil {
			c.vm.Logger().Warn("unable to persist chunk to disk", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}

		// Sign chunk
		// TODO: allow for signing different types of messages
		// TODO: create signer for chunkID
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
		warpMessage := &warp.UnsignedMessage{
			NetworkID:     c.vm.snowCtx.NetworkID,
			SourceChainID: c.vm.snowCtx.ChainID,
			Payload:       digest,
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
		if !c.built.HasID(chunkSignature.Chunk) {
			c.vm.Logger().Warn("dropping useless chunk signature", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}

		// Check that signer is associated with NodeID
		if !bytes.Equal(bls.PublicKeyToBytes(pk), bls.PublicKeyToBytes(chunkSignature.Signer)) {
			c.vm.Logger().Warn("unexpected signer", zap.Stringer("nodeID", nodeID))
			return nil
		}

		// Verify signature
		if !chunkSignature.VerifySignature(c.vm.snowCtx.NetworkID, c.vm.snowCtx.ChainID) {
			c.vm.Logger().Warn("dropping chunk signature with invalid signature", zap.Stringer("nodeID", nodeID))
			return nil
		}

		// Collect signature
		//
		// TODO: add locking
		cw, ok := c.built.Get(chunkSignature.Chunk)
		if !ok {
			// This shouldn't happen because we checked earlier but could
			c.vm.Logger().Warn("dropping untracked chunk", zap.Stringer("nodeID", nodeID))
			return nil
		}
		cw.signatures[utils.ToID(bls.PublicKeyToBytes(chunkSignature.Signer))] = chunkSignature // canonical validator set requires fetching signature by bls public key

		// Count pending weight
		//
		// TODO: add safe math
		// TODO: handle changing validator sets
		validators, _ := c.vm.proposerMonitor.Validators(ctx)
		var (
			weight      uint64 = 0
			totalWeight uint64 = 0
		)
		for _, out := range validators {
			totalWeight += out.Weight
			if out.PublicKey == nil {
				continue
			}
			k := utils.ToID(bls.PublicKeyToBytes(out.PublicKey))
			if _, ok := cw.signatures[k]; ok {
				weight += out.Weight
			}
		}

		// Check if weight is sufficient
		//
		// TODO: only send to next x builders
		// TODO: only send once have a certain weight above 67% or X time until expiry (maximize fee)
		if err := warp.VerifyWeight(weight, totalWeight, 67, 100); err != nil {
			c.vm.Logger().Warn("dropping chunk with insufficient weight", zap.Stringer("chunkID", chunkSignature.Chunk))
			return nil
		}

		// Construct certificate
		canonicalValidators, _, err := c.vm.proposerMonitor.GetCanonicalValidatorSet(ctx)
		if err != nil {
			c.vm.Logger().Warn("cannot get canonical validator set", zap.Error(err))
			return nil
		}
		signers := set.NewBits()
		orderedSignatures := []*bls.Signature{}
		for i, vdr := range canonicalValidators {
			sig, ok := cw.signatures[utils.ToID(bls.PublicKeyToBytes(vdr.PublicKey))]
			if !ok {
				continue
			}
			signers.Add(i)
			orderedSignatures = append(orderedSignatures, sig.Signature)
		}
		aggSignature, err := bls.AggregateSignatures(orderedSignatures)
		if err != nil {
			c.vm.Logger().Warn("cannot generate aggregate signature", zap.Error(err))
			return nil
		}

		// Send certificate to all validators
		//
		// TODO: only send to next x builders
		cert := &chain.ChunkCertificate{
			Chunk: chunkSignature.Chunk,
			Slot:  chunkSignature.Slot,

			Signers:   signers,
			Signature: aggSignature,
		}
		c.PushChunkCertificate(ctx, cert)
	case chunkCertificateMsg:
		cert, err := chain.UnmarshalChunkCertificate(msg[1:])
		if err != nil {
			c.vm.Logger().Warn("dropping chunk gossip from non-validator", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}

		// Ensure certificate isn't too old
		if cert.Slot < c.vm.lastAccepted.StatefulBlock.Timestamp {
			c.vm.Logger().Warn("dropping expired cert", zap.Stringer("nodeID", nodeID))
			return nil
		}

		// Verify certificate using the current validator set
		//
		// TODO: consider re-verifying on some cadence prior to expiry?
		validators, weight, err := c.vm.proposerMonitor.GetCanonicalValidatorSet(ctx)
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
		if err := warp.VerifyWeight(filteredWeight, weight, 67, 100); err != nil {
			return err
		}
		aggrPubKey, err := warp.AggregatePublicKeys(filteredVdrs)
		if err != nil {
			return err
		}
		msg, err := cert.Digest()
		if err != nil {
			return err
		}
		if !bls.Verify(aggrPubKey, cert.Signature, msg) {
			return errors.New("certificate invalid")
		}

		// TODO: fetch chunk if we don't have it from a signer (run down list and sample)

		// Store chunk certificate for building
		c.certs.Add(cert)
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
	return nil
}

func (w *ChunkManager) AppRequestFailed(
	_ context.Context,
	_ ids.NodeID,
	requestID uint32,
) error {
	return nil
}

func (w *ChunkManager) AppResponse(
	_ context.Context,
	_ ids.NodeID,
	requestID uint32,
	response []byte,
) error {
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
}

// Drop all chunks that can no longer be included anymore (may have already been included).
func (c *ChunkManager) SetMin(ctx context.Context, t int64) []ids.ID {
	return c.built.SetMin(t)
}

// TODO: sign own chunks and add to cert
func (c *ChunkManager) PushChunk(ctx context.Context, chunk *chain.Chunk) {
	// TODO: record chunks we sent out to collect signatures
	msg := make([]byte, 1+chunk.Size())
	msg[0] = chunkMsg
	chunkBytes, err := chunk.Marshal()
	if err != nil {
		c.vm.Logger().Warn("failed to marshal chunk", zap.Error(err))
		return
	}
	copy(msg[1:], chunkBytes)
	validators := c.vm.proposerMonitor.GetValidatorSet(ctx, false)
	cw := &chunkWrapper{
		chunk:      chunk,
		signatures: make(map[ids.ID]*chain.ChunkSignature, len(validators)+1),
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
	warpMessage := &warp.UnsignedMessage{
		NetworkID:     c.vm.snowCtx.NetworkID,
		SourceChainID: c.vm.snowCtx.ChainID,
		Payload:       digest,
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
	c.appSender.SendAppGossipSpecific(ctx, c.vm.proposerMonitor.GetValidatorSet(ctx, false), msg) // skips validators we aren't connected to
}

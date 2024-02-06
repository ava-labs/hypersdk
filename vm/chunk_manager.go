package vm

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/hypersdk/chain"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
)

const (
	chunkMsg            uint8 = 0x0
	chunkSignatureMsg   uint8 = 0x1
	chunkCertificateMsg uint8 = 0x2
)

// TODO: move to standalone package
type ChunkManager struct {
	vm *VM

	appSender common.AppSender

	// connected includes all connected nodes, not just those that are validators
	connected set.Set[ids.NodeID]
}

func NewChunkManager(vm *VM) *ChunkManager {
	return &ChunkManager{
		vm: vm,

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
	ok, err := c.vm.proposerMonitor.IsValidator(ctx, nodeID)
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

		// Sign chunk
		// TODO: allow for signing different types of messages
		// TODO: save for chunks we make
		// digest, err := chunk.Digest()
		// if err != nil {
		// 	c.vm.Logger().Warn("unable to produce chunk digest", zap.Stringer("nodeID", nodeID), zap.Error(err))
		// 	return nil
		// }
		// msg := &warp.UnsignedMessage{
		// 	NetworkID:     c.vm.snowCtx.NetworkID,
		// 	SourceChainID: c.vm.snowCtx.ChainID,
		// 	Payload:       digest,
		// }
		// TODO: create signer for chunkID
		sig, err := c.vm.snowCtx.WarpSigner.Sign(msg)
		if err != nil {
			c.vm.Logger().Warn("unable to sign chunk digest", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}

		// Send back signature if valid
		cid, err := chunk.ID()
		if err != nil {
			c.vm.Logger().Warn("unable to get chunkID", zap.Stringer("nodeID", nodeID), zap.Error(err))
			return nil
		}
		sigMsg := &chain.ChunkSignature{
			Chunk: cid,
		}
		c.PushSignature(ctx, nodeID, sigMsg)
	case chunkSignatureMsg:
		// TODO: if chunk creator, collect signatures
	case chunkCertificateMsg:
		// TODO: add to engine for block inclusion
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
	validators, _ := c.vm.proposerMonitor.Validators(ctx)
	// TODO: consider changing to request (for signature)? -> would allow for a job poller style where we could keep sending?
	c.appSender.SendAppGossipSpecific(ctx, set.Of(maps.Keys(validators)...), msg) // skips validators we aren't connected to
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
	validators, _ := c.vm.proposerMonitor.Validators(ctx)
	c.appSender.SendAppGossipSpecific(ctx, set.Of(maps.Keys(validators)...), msg) // skips validators we aren't connected to
}

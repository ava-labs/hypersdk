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

func (*ChunkManager) AppGossip(context.Context, ids.NodeID, []byte) error {
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

func (c *ChunkManager) PushChunk(ctx context.Context, chunk *chain.Chunk) {
	chunkMsg := make([]byte, chunk.Size()+1)
	chunkMsg[0] = 0
	chunkBytes, err := chunk.Marshal()
	if err != nil {
		c.vm.Logger().Warn("failed to marshal chunk", zap.Error(err))
		return
	}
	copy(chunkMsg[1:], chunkBytes)
	validators, _ := c.vm.proposerMonitor.Validators(ctx)
	c.appSender.SendAppGossipSpecific(ctx, set.Of(maps.Keys(validators)...), chunkMsg) // skips validators we aren't connected to
}

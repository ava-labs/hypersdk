package dsmr

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/set"
)

type Client struct {
	sender common.AppSender

	requestID uint32
}

func (c *Client) GetChunk(ctx context.Context, nodeID ids.NodeID, chunkID ids.ID) ([]byte, error) {
	c.sender.SendAppRequest(ctx, set.Of(nodeID), c.requestID, chunkID[:])
	return nil, nil
}

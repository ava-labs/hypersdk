package gossiper

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
)

type Gossiper interface {
	Run(common.AppSender)
	Queue(context.Context)
	Force(context.Context) error // may be triggered by run already
	HandleAppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error
	BlockVerified(int64)
	Done() // wait after stop
}

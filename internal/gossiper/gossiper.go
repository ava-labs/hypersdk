// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossiper

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
)

type Gossiper interface {
	Run(client *p2p.Client)
	Queue(context.Context)
	Force(context.Context) error // may be triggered by run already
	HandleAppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error
	BlockVerified(int64)
	Done() // wait after stop
}

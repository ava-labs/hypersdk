// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossiper

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
)

type Gossiper interface {
	Run()
	TriggerGossip(context.Context) error // may be triggered by run already
	HandleAppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error
	Done() // wait after stop
}

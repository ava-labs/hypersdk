// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossiper

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/network/p2p"
	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"
)

var _ p2p.Handler = (*TxGossipHandler)(nil)

type TxGossipHandler struct {
	p2p.NoOpHandler
	log      logging.Logger
	gossiper Gossiper
}

func NewTxGossipHandler(
	log logging.Logger,
	gossiper Gossiper,
) *TxGossipHandler {
	return &TxGossipHandler{
		log:      log,
		gossiper: gossiper,
	}
}

func (t *TxGossipHandler) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) {
	if err := t.gossiper.HandleAppGossip(ctx, nodeID, msg); err != nil {
		t.log.Warn("handle app gossip failed", zap.Error(err))
	}
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossiper

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/AnomalyFi/hypersdk/chain"
)

type VM interface {
	ChainID() ids.ID
	StopChan() chan struct{}
	Tracer() trace.Tracer
	Mempool() chain.Mempool
	Proposers(ctx context.Context, diff int, depth int) (set.Set[ids.NodeID], error)
	IsValidator(context.Context, ids.NodeID) (bool, error)
	Logger() logging.Logger
	PreferredBlock(context.Context) (*chain.StatelessBlock, error)
	Registry() (chain.ActionRegistry, chain.AuthRegistry)
	NodeID() ids.NodeID
	Rules(int64) chain.Rules
	Submit(ctx context.Context, verify bool, txs []*chain.Transaction) []error
}

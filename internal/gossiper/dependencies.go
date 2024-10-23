// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package gossiper

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/trace"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/codec"
)

type VM interface {
	NetworkID() uint32
	ChainID() ids.ID
	StopChan() chan struct{}
	Tracer() trace.Tracer
	Mempool() chain.Mempool
	GetTargetGossipDuration() time.Duration
	Proposers(ctx context.Context, diff int, depth int) (set.Set[ids.NodeID], error)
	IsValidator(context.Context, ids.NodeID) (bool, error)
	Logger() logging.Logger
	PreferredBlock(context.Context) (*chain.StatefulBlock, error)
	ActionCodec() *codec.TypeParser[chain.Action]
	AuthCodec() *codec.TypeParser[chain.Auth]
	NodeID() ids.NodeID
	Rules(int64) chain.Rules
	Submit(ctx context.Context, verify bool, txs []*chain.Transaction) []error
	GetAuthBatchVerifier(authTypeID uint8, cores int, count int) (chain.AuthBatchVerifier, bool)

	RecordTxsGossiped(int)
	RecordSeenTxsReceived(int)
	RecordTxsReceived(int)
}

// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"context"

	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/AnomalyFi/hypersdk/chain"
)

type VM interface {
	StopChan() chan struct{}
	EngineChan() chan<- common.Message
	PreferredBlock(context.Context) (*chain.StatelessBlock, error)
	Logger() logging.Logger
	Mempool() chain.Mempool
}

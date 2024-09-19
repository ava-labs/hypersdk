// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"context"

	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/logging"

	"github.com/ava-labs/hypersdk/chain"
)

type VM[T chain.RuntimeInterface] interface {
	StopChan() chan struct{}
	EngineChan() chan<- common.Message
	PreferredBlock(context.Context) (*chain.StatefulBlock[T], error)
	Logger() logging.Logger
	Mempool() chain.Mempool[T]
	Rules(int64) chain.Rules
}

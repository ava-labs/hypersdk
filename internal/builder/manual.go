// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package builder

import (
	"context"

	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/logging"
)

var _ Builder = (*Manual)(nil)

type Manual struct {
	engineCh  chan<- common.Message
	logger    logging.Logger
	doneBuild chan struct{}
}

func NewManual(engineCh chan<- common.Message, logger logging.Logger) *Manual {
	return &Manual{
		engineCh:  engineCh,
		logger:    logger,
		doneBuild: make(chan struct{}),
	}
}

func (b *Manual) Start() {
	close(b.doneBuild)
}

// Queue is a no-op in [Manual].
func (*Manual) Queue(context.Context) {}

func (b *Manual) Force(context.Context) error {
	select {
	case b.engineCh <- common.PendingTxs:
	default:
		b.logger.Debug("dropping message to consensus engine")
	}
	return nil
}

func (b *Manual) Done() {
	<-b.doneBuild
}

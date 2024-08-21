// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package integration

import (
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/utils/logging"
	"go.uber.org/zap"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/extension/indexer"
)

var _ indexer.AcceptedSubscriber = (*IntegrationTestSubscriber)(nil)

type IntegrationTestSubscriber struct {
	mu          sync.Mutex
	sendToChan  bool
	blkChan     chan *chain.StatefulBlock
	resultsChan chan []*chain.Result
	log         logging.Logger
}

func NewIntegrationTestSubscriber(
	blkChan chan *chain.StatefulBlock,
	resultsChan chan []*chain.Result,
	log logging.Logger,
) *IntegrationTestSubscriber {
	// By default, does not send block/results to channel
	return &IntegrationTestSubscriber{
		sendToChan:  false,
		blkChan:     blkChan,
		resultsChan: resultsChan,
		log:         log,
	}
}

func (i *IntegrationTestSubscriber) Accepted(
	_ context.Context,
	blk *chain.StatefulBlock,
	results []*chain.Result,
) error {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.sendToChan {
		i.log.Debug("Sending block to channel", zap.Any("Block Height", blk.Hght))
		i.blkChan <- blk
		i.resultsChan <- results
	}
	return nil
}

func (i *IntegrationTestSubscriber) StartSendingToChan() {
	i.mu.Lock()
	i.sendToChan = true
	i.mu.Unlock()
}

func (i *IntegrationTestSubscriber) StopSendingToChan() {
	i.mu.Lock()
	i.sendToChan = false
	i.mu.Unlock()
}

func (i *IntegrationTestSubscriber) SendingToChan() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.sendToChan
}

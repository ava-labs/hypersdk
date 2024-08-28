// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package workload

import (
	"sync"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/extension/externalsubscriber"
)

var (
	_ event.Subscription[*externalsubscriber.ExternalSubscriberSubscriptionData] = (*WorkloadTestSubscriber)(nil)

	_ TestSubscriberServer = (*ExternalSubscriberServer)(nil)
	_ TestSubscriberServer = (*NoOpSubscriberServer)(nil)
)

type WorkloadTestSubscriber struct {
	sync.Mutex
	blkChan      chan *chain.StatefulBlock
	resultsChan  chan []*chain.Result
	channelsOpen bool
}

func NewWorkloadTestSubscriber() *WorkloadTestSubscriber {
	return &WorkloadTestSubscriber{
		blkChan:     make(chan *chain.StatefulBlock),
		resultsChan: make(chan []*chain.Result),
	}
}

func (w *WorkloadTestSubscriber) OpenChannels() {
	w.Lock()
	w.channelsOpen = true
	w.Unlock()
}

func (w *WorkloadTestSubscriber) CloseChannels() {
	w.Lock()
	w.channelsOpen = false
	w.Unlock()
}

func (w *WorkloadTestSubscriber) Accept(e *externalsubscriber.ExternalSubscriberSubscriptionData) error {
	w.Lock()
	defer w.Unlock()

	if !w.channelsOpen {
		return nil
	}
	w.blkChan <- e.Blk
	w.resultsChan <- e.Results
	return nil
}

func (*WorkloadTestSubscriber) Close() error {
	return nil
}

type TestSubscriberServer interface {
	OpenChannels()
	CloseChannels()
	StartHandler()
	StopHandler()
	Get(*require.Assertions, *chain.StatelessBlock)
}

type ExternalSubscriberServer struct {
	grpcHandler        externalsubscriber.GRPCHandler
	workloadSubscriber *WorkloadTestSubscriber
}

func NewExternalSubscriber(grpcHandler externalsubscriber.GRPCHandler, workloadSubscriber *WorkloadTestSubscriber) ExternalSubscriberServer {
	return ExternalSubscriberServer{
		grpcHandler:        grpcHandler,
		workloadSubscriber: workloadSubscriber,
	}
}

func (e ExternalSubscriberServer) OpenChannels() {
	e.workloadSubscriber.OpenChannels()
}

func (e ExternalSubscriberServer) CloseChannels() {
	e.workloadSubscriber.CloseChannels()
}

func (e ExternalSubscriberServer) StartHandler() {
	e.grpcHandler.Start()
}

func (e ExternalSubscriberServer) StopHandler() {
	e.grpcHandler.Stop()
}

func (e ExternalSubscriberServer) Get(require *require.Assertions, expectedBlk *chain.StatelessBlock) {
	blk, results := <-e.workloadSubscriber.blkChan, <-e.workloadSubscriber.resultsChan
	// Compare block IDs
	blkID, err := blk.ID()
	require.NoError(err)
	require.Equal(expectedBlk.ID(), blkID)
	// Compare results
	require.Equal(expectedBlk.Results(), results)
}

type NoOpSubscriberServer struct{}

func (*NoOpSubscriberServer) CloseChannels() {}

func (*NoOpSubscriberServer) Get(*require.Assertions, *chain.StatelessBlock) {}

func (*NoOpSubscriberServer) OpenChannels() {}

func (*NoOpSubscriberServer) StartHandler() {}

func (*NoOpSubscriberServer) StopHandler() {}

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txindexer

import (
	"sync"

	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/internal/cache"
	"github.com/ava-labs/hypersdk/vm"
)

const (
	Name      = "blockindexer"
	Namespace = "blockindexer"
)

var (
	_ event.SubscriptionFactory[*chain.StatefulBlock] = (*subscriptionFactory)(nil)
	_ event.Subscription[*chain.StatefulBlock]        = (*indexer)(nil)
)

type Config struct {
	Enabled             bool `json:"enabled"`
	AcceptedBlockWindow int  `json:"acceptedBlockWindow"`
}

func NewDefaultConfig() Config {
	return Config{
		Enabled:             true,
		AcceptedBlockWindow: 1_000,
	}
}

func With() vm.Option {
	return vm.NewOption(Namespace, NewDefaultConfig(), OptionFunc)
}

func OptionFunc(v *vm.VM, config Config) error {
	if !config.Enabled {
		return nil
	}

	acceptedBlocksByID, err := cache.NewFIFO[ids.ID, *chain.StatefulBlock](config.AcceptedBlockWindow)
	if err != nil {
		return err
	}
	acceptedBlocksByHeight, err := cache.NewFIFO[uint64, ids.ID](config.AcceptedBlockWindow)
	if err != nil {
		return err
	}

	indexer := &indexer{
		acceptedBlocksByID:     acceptedBlocksByID,
		acceptedBlocksByHeight: acceptedBlocksByHeight,
	}

	subscriptionFactory := &subscriptionFactory{
		indexer: indexer,
	}

	apiFactory := &apiFactory{
		path:    Endpoint,
		name:    Name,
		indexer: indexer,
	}

	vm.WithBlockSubscriptions(subscriptionFactory)(v)
	vm.WithVMAPIs(apiFactory)(v)

	return nil
}

type subscriptionFactory struct {
	indexer *indexer
}

func (s *subscriptionFactory) New() (event.Subscription[*chain.StatefulBlock], error) {
	return s.indexer, nil
}

type indexer struct {
	lock                   sync.RWMutex
	acceptedBlocksByID     *cache.FIFO[ids.ID, *chain.StatefulBlock]
	acceptedBlocksByHeight *cache.FIFO[uint64, ids.ID]
	latestBlock            *chain.StatefulBlock
}

func (i *indexer) Accept(blk *chain.StatefulBlock) error {
	i.lock.Lock()
	defer i.lock.Unlock()

	i.latestBlock = blk
	i.acceptedBlocksByID.Put(blk.ID(), blk)
	i.acceptedBlocksByHeight.Put(blk.Height(), blk.ID())
	return nil
}

func (i *indexer) GetBlock(blkID ids.ID) (*chain.StatefulBlock, bool) {
	i.lock.RLock()
	defer i.lock.RUnlock()

	return i.acceptedBlocksByID.Get(blkID)
}

func (i *indexer) GetBlockByHeight(height uint64) (*chain.StatefulBlock, bool) {
	i.lock.RLock()
	defer i.lock.RUnlock()

	blkID, ok := i.acceptedBlocksByHeight.Get(height)
	if !ok {
		return nil, false
	}
	return i.acceptedBlocksByID.Get(blkID)
}

func (i *indexer) GetLatestBlock() (*chain.StatefulBlock, bool) {
	i.lock.RLock()
	defer i.lock.RUnlock()

	return i.latestBlock, i.latestBlock != nil
}

func (*indexer) Close() error {
	return nil
}

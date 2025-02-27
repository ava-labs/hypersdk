// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"path/filepath"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/vm"
)

const (
	Name      = "indexer"
	Namespace = "indexer"
)

var _ event.SubscriptionFactory[*chain.ExecutedBlock] = (*subscriptionFactory)(nil)

type Config struct {
	Enabled     bool   `json:"enabled"`
	BlockWindow uint64 `json:"blockWindow"`
}

func NewDefaultConfig() Config {
	return Config{
		Enabled:     true,
		BlockWindow: 1024,
	}
}

func With() vm.Option {
	return vm.NewOption(Namespace, NewDefaultConfig(), OptionFunc)
}

func OptionFunc(v api.VM, config Config) (vm.Opt, error) {
	if !config.Enabled {
		return vm.NewOpt(), nil
	}
	indexerPath := filepath.Join(v.GetDataDir(), Namespace)
	indexer, err := NewIndexer(indexerPath, v.GetParser(), config.BlockWindow)
	if err != nil {
		return nil, err
	}

	subscriptionFactory := &subscriptionFactory{
		indexer: indexer,
	}

	apiFactory := &apiFactory{
		path:    Endpoint,
		name:    Name,
		indexer: indexer,
	}

	return vm.NewOpt(
		vm.WithBlockSubscriptions(subscriptionFactory),
		vm.WithVMAPIs(apiFactory),
	), nil
}

type subscriptionFactory struct {
	indexer *Indexer
}

func (s *subscriptionFactory) New() (event.Subscription[*chain.ExecutedBlock], error) {
	return s.indexer, nil
}

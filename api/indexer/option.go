// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package indexer

import (
	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
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

func With() api.Option {
	return api.NewOption(Namespace, NewDefaultConfig(), OptionFunc)
}

func OptionFunc(v api.VM, config Config) (api.Opt, error) {
	if !config.Enabled {
		return api.NewOpt(), nil
	}
	indexer, err := NewIndexer(v.GetDataDir(Namespace), v, config.BlockWindow)
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

	return api.NewOpt(
		api.WithBlockSubscriptions(subscriptionFactory),
		api.WithVMAPIs(apiFactory),
	), nil
}

type subscriptionFactory struct {
	indexer *Indexer
}

func (s *subscriptionFactory) New() (event.Subscription[*chain.ExecutedBlock], error) {
	return s.indexer, nil
}

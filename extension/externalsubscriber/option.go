// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package externalsubscriber

import (
	"context"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
)

const (
	Namespace = "externalSubscriber"
)

type Config struct {
	Enabled       bool   `json:"enabled"`
	ServerAddress string `json:"serverAddress"`
}

func NewDefaultConfig() Config {
	return Config{}
}

func With() api.Option {
	return api.NewOption(Namespace, NewDefaultConfig(), OptionFunc)
}

func OptionFunc(v api.VM, config Config) (api.Opt, error) {
	if !config.Enabled {
		return api.NewOpt(), nil
	}
	server, err := NewExternalSubscriberClient(
		context.TODO(),
		v.Logger(),
		config.ServerAddress,
		v.GetGenesisBytes(),
	)
	if err != nil {
		return nil, err
	}

	blockSubscription := event.SubscriptionFuncFactory[*chain.ExecutedBlock]{
		AcceptF: func(blk *chain.ExecutedBlock) error {
			return server.Accept(blk)
		},
	}

	return api.WithBlockSubscriptions(blockSubscription), nil
}

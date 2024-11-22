// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package externalsubscriber

import (
	"context"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/vm"
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

func With() vm.Option {
	return vm.NewOption(Namespace, NewDefaultConfig(), OptionFunc)
}

func OptionFunc(v api.VM, config Config) (vm.Opt, error) {
	if !config.Enabled {
		return vm.NewOpt(), nil
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
		AcceptF: func(ctx context.Context, blk *chain.ExecutedBlock) error {
			return server.Accept(ctx, blk)
		},
	}

	return vm.WithBlockSubscriptions(blockSubscription), nil
}

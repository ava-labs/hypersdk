// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package externalsubscriber

import (
	"context"

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
	return vm.NewOptionWithConfig(Namespace, NewDefaultConfig(), OptionFunc)
}

func OptionFunc(v *vm.VM, config Config) error {
	if !config.Enabled {
		return nil
	}
	server, err := NewExternalSubscriberClient(
		context.TODO(),
		v.Logger(),
		config.ServerAddress,
		v.GenesisBytes,
	)
	if err != nil {
		return err
	}

	blockSubscription := event.SubscriptionFuncFactory[*chain.StatelessBlock]{
		AcceptF: func(blk *chain.StatelessBlock) error {
			return server.Accept(blk)
		},
	}

	vm.WithBlockSubscriptions(blockSubscription)(v)
	return nil
}

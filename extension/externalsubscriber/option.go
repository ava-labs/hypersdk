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

func With[T chain.RuntimeInterface]() vm.Option[T] {
	return vm.NewOption[T](Namespace, NewDefaultConfig(), OptionFunc[T])
}

func OptionFunc[T chain.RuntimeInterface](v *vm.VM[T], config Config) error {
	if !config.Enabled {
		return nil
	}
	server, err := NewExternalSubscriberClient[T](
		context.TODO(),
		v.Logger(),
		config.ServerAddress,
		v.GenesisBytes,
	)
	if err != nil {
		return err
	}

	blockSubscription := event.SubscriptionFuncFactory[*chain.StatefulBlock[T]]{
		AcceptF: func(blk *chain.StatefulBlock[T]) error {
			return server.Accept(blk)
		},
	}

	vm.WithBlockSubscriptions[T](blockSubscription)(v)
	return nil
}

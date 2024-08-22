// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package externalsubscriber

import (
	"context"
	"encoding/json"

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

func NewConfig() Config {
	return Config{
		Enabled: false,
	}
}

func With() vm.Option {
	return vm.NewOption(Namespace, OptionFunc)
}

func OptionFunc(v *vm.VM, configBytes []byte) error {
	config := NewConfig()
	if len(configBytes) > 0 {
		if err := json.Unmarshal(configBytes, &config); err != nil {
			return err
		}
	}
	if !config.Enabled {
		return nil
	}
	server, err := NewExternalSubscriberClient(context.TODO(), v.Logger(), config.ServerAddress, v.SnowCtx.NetworkID, v.SnowCtx.ChainID, v.GenesisBytes)
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

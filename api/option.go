// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package api

import (
	"encoding/json"
	"fmt"

	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
)

type Options struct {
	Builder                        bool
	Gossiper                       bool
	BlockSubscriptionFactories     []event.SubscriptionFactory[*chain.ExecutedBlock]
	VMAPIHandlerFactories          []HandlerFactory[VM]
	TxRemovedSubscriptionFactories []event.SubscriptionFactory[TxRemovedEvent]
}

type optionFunc func(vm VM, configBytes []byte) (Opt, error)

type OptionFunc[T any] func(vm VM, config T) (Opt, error)

type Option struct {
	Namespace  string
	OptionFunc optionFunc
}

func newOptionWithBytes(namespace string, optionFunc optionFunc) Option {
	return Option{
		Namespace:  namespace,
		OptionFunc: optionFunc,
	}
}

// NewOption returns an option with:
// 1) A namespace to define the key in the VM's JSON config that should be supplied to this option
// 2) A default config value the VM will directly unmarshal into
// 3) An option function that takes the VM and resulting config value as arguments
func NewOption[T any](namespace string, defaultConfig T, optionFunc OptionFunc[T]) Option {
	config := defaultConfig
	configOptionFunc := func(vm VM, configBytes []byte) (Opt, error) {
		if len(configBytes) > 0 {
			if err := json.Unmarshal(configBytes, &config); err != nil {
				return nil, fmt.Errorf("failed to unmarshal %q config %q: %w", namespace, string(configBytes), err)
			}
		}

		return optionFunc(vm, config)
	}
	return newOptionWithBytes(namespace, configOptionFunc)
}

func withBuilder() Opt {
	return newFuncOption(func(o *Options) {
		o.Builder = true
	})
}

func withGossiper() Opt {
	return newFuncOption(func(o *Options) {
		o.Gossiper = true
	})
}

func WithManual() Option {
	return NewOption[struct{}](
		"manual",
		struct{}{},
		func(_ VM, _ struct{}) (Opt, error) {
			return newFuncOption(func(o *Options) {
				withBuilder().Apply(o)
				withGossiper().Apply(o)
			}), nil
		},
	)
}

func WithBlockSubscriptions(subscriptions ...event.SubscriptionFactory[*chain.ExecutedBlock]) Opt {
	return newFuncOption(func(o *Options) {
		o.BlockSubscriptionFactories = append(o.BlockSubscriptionFactories, subscriptions...)
	})
}

func WithVMAPIs(apiHandlerFactories ...HandlerFactory[VM]) Opt {
	return newFuncOption(func(o *Options) {
		o.VMAPIHandlerFactories = append(o.VMAPIHandlerFactories, apiHandlerFactories...)
	})
}

func WithTxRemovedSubscriptions(subscriptions ...event.SubscriptionFactory[TxRemovedEvent]) Opt {
	return newFuncOption(func(o *Options) {
		o.TxRemovedSubscriptionFactories = append(o.TxRemovedSubscriptionFactories, subscriptions...)
	})
}

type Opt interface {
	Apply(*Options)
}

// NewOpt mixes a list of Opt in a new one Opt.
func NewOpt(opts ...Opt) Opt {
	return newFuncOption(func(o *Options) {
		for _, opt := range opts {
			opt.Apply(o)
		}
	})
}

type funcOption struct {
	f func(*Options)
}

func (fdo *funcOption) Apply(do *Options) {
	fdo.f(do)
}

func newFuncOption(f func(*Options)) *funcOption {
	return &funcOption{
		f: f,
	}
}

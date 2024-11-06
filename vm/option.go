// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"encoding/json"
	"fmt"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
)

type Options struct {
	builder                    bool
	gossiper                   bool
	blockSubscriptionFactories []event.SubscriptionFactory[*chain.ExecutedBlock]
	vmAPIHandlerFactories      []api.HandlerFactory[api.VM]
}

type optionFunc func(vm api.VM, configBytes []byte) (Opt, error)

type OptionFunc[T any] func(vm api.VM, config T) (Opt, error)

type Option struct {
	Namespace  string
	optionFunc optionFunc
}

func newOptionWithBytes(namespace string, optionFunc optionFunc) Option {
	return Option{
		Namespace:  namespace,
		optionFunc: optionFunc,
	}
}

// NewOption returns an option with:
// 1) A namespace to define the key in the VM's JSON config that should be supplied to this option
// 2) A default config value the VM will directly unmarshal into
// 3) An option function that takes the VM and resulting config value as arguments
func NewOption[T any](namespace string, defaultConfig T, optionFunc OptionFunc[T]) Option {
	config := defaultConfig
	configOptionFunc := func(vm api.VM, configBytes []byte) (Opt, error) {
		if len(configBytes) > 0 {
			if err := json.Unmarshal(configBytes, &config); err != nil {
				return nil, fmt.Errorf("failed to unmarshal %q config %q: %w", namespace, string(configBytes), err)
			}
		}

		return optionFunc(vm, config)
	}
	return newOptionWithBytes(namespace, configOptionFunc)
}

func WithBuilder() Opt {
	return newFuncOption(func(o *Options) {
		o.builder = true
	})
}

func WithGossiper() Opt {
	return newFuncOption(func(o *Options) {
		o.gossiper = true
	})
}

func WithManual() Option {
	return NewOption[struct{}](
		"manual",
		struct{}{},
		func(_ api.VM, _ struct{}) (Opt, error) {
			return newFuncOption(func(o *Options) {
				WithBuilder().apply(o)
				WithGossiper().apply(o)
			}), nil
		},
	)
}

func WithBlockSubscriptions(subscriptions ...event.SubscriptionFactory[*chain.ExecutedBlock]) Opt {
	return newFuncOption(func(o *Options) {
		o.blockSubscriptionFactories = append(o.blockSubscriptionFactories, subscriptions...)
	})
}

func WithVMAPIs(apiHandlerFactories ...api.HandlerFactory[api.VM]) Opt {
	return newFuncOption(func(o *Options) {
		o.vmAPIHandlerFactories = append(o.vmAPIHandlerFactories, apiHandlerFactories...)
	})
}

type Opt interface {
	apply(*Options)
}

// NewOpt mixes a list of Opt in a new one Opt.
func NewOpt(opts ...Opt) Opt {
	return newFuncOption(func(o *Options) {
		for _, opt := range opts {
			opt.apply(o)
		}
	})
}

type funcOption struct {
	f func(*Options)
}

func (fdo *funcOption) apply(do *Options) {
	fdo.f(do)
}

func newFuncOption(f func(*Options)) *funcOption {
	return &funcOption{
		f: f,
	}
}

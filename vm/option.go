// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"encoding/json"
	"fmt"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/builder"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/gossiper"
)

type RegisterFunc func(*VM)

type OptionFunc func(vm *VM, configBytes []byte) error

type OptionConfigFunc[T any] func(vm *VM, config T) error

type Option struct {
	Namespace  string
	OptionFunc OptionFunc
}

func NewOptionWithBytes(namespace string, optionFunc OptionFunc) Option {
	return Option{
		Namespace:  namespace,
		OptionFunc: optionFunc,
	}
}

// NewOptionWithConfig returns an option function that unmarshals the config bytes into the supplied default
// config and calls the supplied option function with the config.
func NewOptionWithConfig[T any](namespace string, defaultConfig T, optionFunc OptionConfigFunc[T]) Option {
	config := defaultConfig
	configOptionFunc := func(vm *VM, configBytes []byte) error {
		if len(configBytes) > 0 {
			if err := json.Unmarshal(configBytes, &config); err != nil {
				return fmt.Errorf("failed to unmarshal %q config %q: %w", namespace, string(configBytes), err)
			}
		}

		return optionFunc(vm, config)
	}
	return NewOptionWithBytes(namespace, configOptionFunc)
}

func WithBuilder() RegisterFunc {
	return func(vm *VM) {
		vm.builder = builder.NewManual(vm)
	}
}

func WithGossiper() RegisterFunc {
	return func(vm *VM) {
		vm.gossiper = gossiper.NewManual(vm)
	}
}

func WithManual() Option {
	return NewOptionWithBytes(
		"manual",
		func(vm *VM, _ []byte) error {
			WithBuilder()(vm)
			WithGossiper()(vm)
			return nil
		},
	)
}

func WithBlockSubscriptions(subscriptions ...event.SubscriptionFactory[*chain.StatelessBlock]) RegisterFunc {
	return func(vm *VM) {
		vm.blockSubscriptionFactories = append(vm.blockSubscriptionFactories, subscriptions...)
	}
}

func WithVMAPIs(apiHandlerFactories ...api.HandlerFactory[api.VM]) RegisterFunc {
	return func(vm *VM) {
		vm.vmAPIHandlerFactories = append(vm.vmAPIHandlerFactories, apiHandlerFactories...)
	}
}

func WithTxRemovedSubscriptions(subscriptions ...event.SubscriptionFactory[TxRemovedEvent]) RegisterFunc {
	return func(vm *VM) {
		vm.txRemovedSubscriptionFactories = append(vm.txRemovedSubscriptionFactories, subscriptions...)
	}
}

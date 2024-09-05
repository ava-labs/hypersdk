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

type optionFunc func(vm *VM, configBytes []byte) error

type OptionFunc[T any] func(vm *VM, config T) error

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
	configOptionFunc := func(vm *VM, configBytes []byte) error {
		if len(configBytes) > 0 {
			if err := json.Unmarshal(configBytes, &config); err != nil {
				return fmt.Errorf("failed to unmarshal %q config %q: %w", namespace, string(configBytes), err)
			}
		}

		return optionFunc(vm, config)
	}
	return newOptionWithBytes(namespace, configOptionFunc)
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
	return NewOption[struct{}](
		"manual",
		struct{}{},
		func(vm *VM, _ struct{}) error {
			WithBuilder()(vm)
			WithGossiper()(vm)
			return nil
		},
	)
}

func WithBlockSubscriptions(subscriptions ...event.SubscriptionFactory[*chain.StatefulBlock]) RegisterFunc {
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

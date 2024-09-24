// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"encoding/json"
	"fmt"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/internal/builder"
	"github.com/ava-labs/hypersdk/internal/gossiper"
)

type RegisterFunc[T chain.PendingView] func(*VM[T])

type optionFunc[T chain.PendingView] func(vm *VM[T], configBytes []byte) error

type OptionFunc[T chain.PendingView, U any] func(vm *VM[T], config U) error

type Option[T chain.PendingView] struct {
	Namespace  string
	optionFunc optionFunc[T]
}

func newOptionWithBytes[T chain.PendingView](namespace string, optionFunc optionFunc[T]) Option[T] {
	return Option[T]{
		Namespace:  namespace,
		optionFunc: optionFunc,
	}
}

// NewOption returns an option with:
// 1) A namespace to define the key in the VM's JSON config that should be supplied to this option
// 2) A default config value the VM will directly unmarshal into
// 3) An option function that takes the VM and resulting config value as arguments
func NewOption[T chain.PendingView, U any](namespace string, defaultConfig U, optionFunc OptionFunc[T, U]) Option[T] {
	config := defaultConfig
	configOptionFunc := func(vm *VM[T], configBytes []byte) error {
		if len(configBytes) > 0 {
			if err := json.Unmarshal(configBytes, &config); err != nil {
				return fmt.Errorf("failed to unmarshal %q config %q: %w", namespace, string(configBytes), err)
			}
		}

		return optionFunc(vm, config)
	}
	return newOptionWithBytes(namespace, configOptionFunc)
}

func WithBuilder[T chain.PendingView]() RegisterFunc[T] {
	return func(vm *VM[T]) {
		vm.builder = builder.NewManual[T](vm)
	}
}

func WithGossiper[T chain.PendingView]() RegisterFunc[T] {
	return func(vm *VM[T]) {
		vm.gossiper = gossiper.NewManual[T](vm)
	}
}

func WithManual[T chain.PendingView]() Option[T] {
	return NewOption(
		"manual",
		struct{}{},
		func(vm *VM[T], _ struct{}) error {
			WithBuilder[T]()(vm)
			WithGossiper[T]()(vm)
			return nil
		},
	)
}

func WithBlockSubscriptions[T chain.PendingView](subscriptions ...event.SubscriptionFactory[*chain.StatefulBlock[T]]) RegisterFunc[T] {
	return func(vm *VM[T]) {
		vm.blockSubscriptionFactories = append(vm.blockSubscriptionFactories, subscriptions...)
	}
}

func WithVMAPIs[T chain.PendingView](apiHandlerFactories ...api.HandlerFactory[api.VM[T]]) RegisterFunc[T] {
	return func(vm *VM[T]) {
		vm.vmAPIHandlerFactories = append(vm.vmAPIHandlerFactories, apiHandlerFactories...)
	}
}

func WithTxRemovedSubscriptions[T chain.PendingView](subscriptions ...event.SubscriptionFactory[TxRemovedEvent]) RegisterFunc[T] {
	return func(vm *VM[T]) {
		vm.txRemovedSubscriptionFactories = append(vm.txRemovedSubscriptionFactories, subscriptions...)
	}
}

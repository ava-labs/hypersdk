// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/builder"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/gossiper"
)

type OptionFunc func(*VM, []byte) error

type Option struct {
	Namespace  string
	OptionFunc OptionFunc
}

func NewOption(namespace string, optionFunc OptionFunc) Option {
	return Option{
		Namespace:  namespace,
		OptionFunc: optionFunc,
	}
}

func WithManualBuilder() Option {
	return NewOption(
		"manualBuilder",
		func(vm *VM, _ []byte) error {
			vm.builder = builder.NewManual(vm)
			return nil
		},
	)
}

func WithManualGossiper() Option {
	return NewOption(
		"manualGossiper",
		func(vm *VM, _ []byte) error {
			vm.gossiper = gossiper.NewManual(vm)
			return nil
		},
	)
}

func WithBlockSubscriptions(subscriptions ...event.SubscriptionFactory[*chain.StatelessBlock]) OptionFunc {
	return func(vm *VM, _ []byte) error {
		vm.blockSubscriptionFactories = append(vm.blockSubscriptionFactories, subscriptions...)
		return nil
	}
}

func WithVMAPIs(apiHandlerFactories ...api.HandlerFactory[api.VM]) OptionFunc {
	return func(vm *VM, _ []byte) error {
		vm.vmAPIHandlerFactories = append(vm.vmAPIHandlerFactories, apiHandlerFactories...)
		return nil
	}
}

func WithControllerAPIs(apiHandlerFactories ...api.HandlerFactory[Controller]) OptionFunc {
	return func(vm *VM, _ []byte) error {
		vm.controllerAPIHandlerFactories = append(vm.controllerAPIHandlerFactories, apiHandlerFactories...)
		return nil
	}
}

func WithTxRemovedSubscriptions(subscriptions ...event.SubscriptionFactory[TxRemovedEvent]) OptionFunc {
	return func(vm *VM, _ []byte) error {
		vm.txRemovedSubscriptionFactories = append(vm.txRemovedSubscriptionFactories, subscriptions...)
		return nil
	}
}

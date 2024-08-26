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

type VMFunc func(*VM)

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

func WithBlockSubscriptions(subscriptions ...event.SubscriptionFactory[*chain.StatelessBlock]) VMFunc {
	return func(vm *VM) {
		vm.blockSubscriptionFactories = append(vm.blockSubscriptionFactories, subscriptions...)
	}
}

func WithVMAPIs(apiHandlerFactories ...api.HandlerFactory[api.VM]) VMFunc {
	return func(vm *VM) {
		vm.vmAPIHandlerFactories = append(vm.vmAPIHandlerFactories, apiHandlerFactories...)
	}
}

func WithControllerAPIs(apiHandlerFactories ...api.HandlerFactory[Controller]) VMFunc {
	return func(vm *VM) {
		vm.controllerAPIHandlerFactories = append(vm.controllerAPIHandlerFactories, apiHandlerFactories...)
	}
}

func WithTxRemovedSubscriptions(subscriptions ...event.SubscriptionFactory[TxRemovedEvent]) VMFunc {
	return func(vm *VM) {
		vm.txRemovedSubscriptionFactories = append(vm.txRemovedSubscriptionFactories, subscriptions...)
	}
}

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

type Option func(*VM)

func WithManualBuilder() Option {
	return func(vm *VM) {
		vm.builder = builder.NewManual(vm)
	}
}

func WithManualGossiper() Option {
	return func(vm *VM) {
		vm.gossiper = gossiper.NewManual(vm)
	}
}

func WithBlockSubscriptions(subscriptions ...event.SubscriptionFactory[*chain.StatelessBlock]) Option {
	return func(vm *VM) {
		vm.blockSubscriptionFactories = append(vm.blockSubscriptionFactories, subscriptions...)
	}
}

func WithVMAPIs(apiHandlerFactories ...api.HandlerFactory[api.VM]) Option {
	return func(vm *VM) {
		vm.vmAPIHandlerFactories = append(vm.vmAPIHandlerFactories, apiHandlerFactories...)
	}
}

func WithControllerAPIs(apiHandlerFactories ...api.HandlerFactory[Controller]) Option {
	return func(vm *VM) {
		vm.controllerAPIHandlerFactories = append(vm.controllerAPIHandlerFactories, apiHandlerFactories...)
	}
}

func WithTxRemovedSubscriptions(subscriptions ...event.SubscriptionFactory[TxRemovedEvent]) Option {
	return func(vm *VM) {
		vm.txRemovedSubscriptionFactories = append(vm.txRemovedSubscriptionFactories, subscriptions...)
	}
}

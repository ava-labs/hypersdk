// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/hypersdk/builder"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/gossiper"
	"github.com/ava-labs/hypersdk/rpc"
	"github.com/ava-labs/hypersdk/vm/event"
)

type Option[T Controller] func(*VM[T])

func WithManualBuilder[T Controller]() Option[T] {
	return func(vm *VM[T]) {
		vm.builder = builder.NewManual(vm)
	}
}

func WithManualGossiper[T Controller]() Option[T] {
	return func(vm *VM[T]) {
		vm.gossiper = gossiper.NewManual(vm)
	}
}

func WithBlockSubscriptions[T Controller](subscriptions ...event.SubscriptionFactory[*chain.StatelessBlock]) Option[T] {
	return func(vm *VM[T]) {
		vm.blockSubscriptionFactories = subscriptions
	}
}

func WithVMAPIs[T Controller](apiHandlerFactories ...rpc.HandlerFactory[rpc.VM]) Option[T] {
	return func(vm *VM[T]) {
		vm.vmAPIHandlerFactories = apiHandlerFactories
	}
}

func WithControllerAPIs[T Controller](apiHandlerFactories ...rpc.HandlerFactory[T]) Option[T] {
	return func(vm *VM[T]) {
		vm.controllerAPIHandlerFactories = apiHandlerFactories
	}
}

func WithTxRemovedSubscriptions[T Controller](subscriptions ...event.SubscriptionFactory[TxRemovedEvent]) Option[T] {
	return func(vm *VM[T]) {
		vm.txRemovedSubscriptionFactories = subscriptions
	}
}

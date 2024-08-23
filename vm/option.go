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

type Option[T any] func(*VM[T]) error

func WithManualBuilder[T any]() Option[T] {
	return func(vm *VM[T]) error {
		vm.builder = builder.NewManual(vm)
		return nil
	}
}

func WithManualGossiper[T any]() Option[T] {
	return func(vm *VM[T]) error {
		vm.gossiper = gossiper.NewManual(vm)
		return nil
	}
}

func WithBlockSubscriptions[T any](subscriptions ...event.SubscriptionFactory[*chain.StatelessBlock]) Option[T] {
	return func(vm *VM[T]) error {
		vm.blockSubscriptionFactories = append(vm.blockSubscriptionFactories, subscriptions...)
		return nil
	}
}

func WithVMAPIs[T any](apiHandlerFactories ...api.HandlerFactory[api.VM]) Option[T] {
	return func(vm *VM[T]) error {
		vm.vmAPIHandlerFactories = append(vm.vmAPIHandlerFactories, apiHandlerFactories...)
		return nil
	}
}

func WithControllerAPIs[T any](apiHandlerFactories ...api.HandlerFactory[Controller]) Option[T] {
	return func(vm *VM[T]) error {
		vm.controllerAPIHandlerFactories = append(vm.controllerAPIHandlerFactories, apiHandlerFactories...)
		return nil
	}
}

func WithTxRemovedSubscriptions[T any](subscriptions ...event.SubscriptionFactory[TxRemovedEvent]) Option[T] {
	return func(vm *VM[T]) error {
		vm.txRemovedSubscriptionFactories = append(vm.txRemovedSubscriptionFactories, subscriptions...)
		return nil
	}
}

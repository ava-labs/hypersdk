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

type RegisterFunc func(*VM)

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
	return NewOption(
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

// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/avalanchego/ids"

	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/builder"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/event"
	"github.com/ava-labs/hypersdk/gossiper"
)

type Option func(*VM) error

func WithManualBuilder() Option {
	return func(vm *VM) error {
		vm.builder = builder.NewManual(vm)
		return nil
	}
}

func WithManualGossiper() Option {
	return func(vm *VM) error {
		vm.gossiper = gossiper.NewManual(vm)
		return nil
	}
}

func WithBlockSubscriptions(subscriptions ...event.SubscriptionFactory[*chain.StatelessBlock]) Option {
	return func(vm *VM) error {
		vm.blockSubscriptionFactories = append(vm.blockSubscriptionFactories, subscriptions...)
		return nil
	}
}

func WithVMAPIs(apiHandlerFactories ...api.HandlerFactory[api.VM]) Option {
	return func(vm *VM) error {
		vm.vmAPIHandlerFactories = append(vm.vmAPIHandlerFactories, apiHandlerFactories...)
		return nil
	}
}

func WithControllerAPIs(apiHandlerFactories ...api.HandlerFactory[Controller]) Option {
	return func(vm *VM) error {
		vm.controllerAPIHandlerFactories = append(vm.controllerAPIHandlerFactories, apiHandlerFactories...)
		return nil
	}
}

func WithTxRemovedSubscriptions(subscriptions ...event.SubscriptionFactory[TxRemovedEvent]) Option {
	return func(vm *VM) error {
		vm.txRemovedSubscriptionFactories = append(vm.txRemovedSubscriptionFactories, subscriptions...)
		return nil
	}
}

func WithCustomGenesisLoader(loadGenesis func(genesisBytes []byte) (Genesis, error)) Option {
	return func(vm *VM) error {
		vm.genesisAndRuleHandler.LoadGenesis = loadGenesis
		return nil
	}
}

func WithCustomRuleLoader(loadRules func(genesisBytes []byte, upgradeBytes []byte, networkID uint32, chainID ids.ID) (RuleFactory, error)) Option {
	return func(vm *VM) error {
		vm.genesisAndRuleHandler.LoadRules = loadRules
		return nil
	}
}

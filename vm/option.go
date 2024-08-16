// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/hypersdk/builder"
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

func WithRuleParser(rp RuleParser) Option {
	return func(vm *VM) {
		vm.ruleParser = rp
	}
}

func WithGenesisParser(gp GenesisParser) Option {
	return func(vm *VM) {
		vm.genesisParser = gp
	}
}

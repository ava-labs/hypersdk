// Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package vm

import (
	"github.com/ava-labs/hypersdk/builder"
	"github.com/ava-labs/hypersdk/gossiper"
)

type Option func(*VM)

func WithBlockBuilder(blockBuilder builder.Builder) Option {
	return func(vm *VM) {
		vm.
	}
}

func WithTxGossiper(txGossiper gossiper.Gossiper) Option {
	return OptionFunc(func(_ *VM, options *Options) {
		options.TxGossiper = txGossiper
	})
}

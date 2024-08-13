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

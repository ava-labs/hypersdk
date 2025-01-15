// Copyright (C) 2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package tieredstorage

import (
	"github.com/ava-labs/hypersdk/api"
	"github.com/ava-labs/hypersdk/chain"
	"github.com/ava-labs/hypersdk/vm"
)

const Namespace = "tieredStorage"

type Config struct {
	Epsilon                uint64 `json:"epsilon"`
	StorageReadKeyRefund   uint64 `json:"storageReadKeyRefund"`
	StorageReadValueRefund uint64 `json:"storageReadValueRefund"`
}

func NewDefaultConfig() Config {
	return Config{
		Epsilon:                100,
		StorageReadKeyRefund:   1,
		StorageReadValueRefund: 1,
	}
}

func With() vm.Option {
	return vm.NewOption(Namespace, NewDefaultConfig(), OptionFunc)
}

func OptionFunc(_ api.VM, config Config) (vm.Opt, error) {
	return vm.NewOpt(
		vm.WithChainOptions(
			chain.WithExecutionShim(
				ExecutionShim(config),
			),
			chain.WithExportStateDiffFunc(ExportStateDiff),
			chain.WithRefundFunc(Refund),
			chain.WithDimsModifierFunc(FeeManagerModifier),
			chain.WithResultModifierFunc(ResultModifier),
		),
		vm.WithExecutionShim(
			ExecutionShim(config),
		),
	), nil
}

func ExecutionShim(config Config) *Shim {
	return &Shim{
		config: config,
	}
}
